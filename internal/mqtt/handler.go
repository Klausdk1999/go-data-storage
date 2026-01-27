package mqtt

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"log"
	"strings"
	"time"

	"data-storage/internal/db"
	"data-storage/internal/models"
)

// TTNUplinkMessage matches the structure from TTN MQTT messages
type TTNUplinkMessage struct {
	EndDeviceIDs struct {
		DeviceID      string `json:"device_id"`
		ApplicationID struct {
			ApplicationID string `json:"application_id"`
		} `json:"application_ids"`
		DevEUI string `json:"dev_eui"`
	} `json:"end_device_ids"`
	ReceivedAt string `json:"received_at"`
	UplinkMessage struct {
		FPort      int    `json:"f_port"`
		FCnt       int    `json:"f_cnt"`
		FRMPayload string `json:"frm_payload"` // Base64 encoded
		RXMetadata []struct {
			GatewayIDs struct {
				GatewayID string `json:"gateway_id"`
			} `json:"gateway_ids"`
			RSSI int     `json:"rssi"`
			SNR  float64 `json:"snr"`
		} `json:"rx_metadata"`
		Settings struct {
			DataRate struct {
				LoRA struct {
					SpreadingFactor int `json:"spreading_factor"`
				} `json:"lora"`
			} `json:"data_rate"`
			Frequency string `json:"frequency"`
		} `json:"settings"`
	} `json:"uplink_message"`
}

// PayloadFormat identifies which device format was detected
type PayloadFormat string

const (
	FormatLegacy8Byte  PayloadFormat = "legacy_8byte"   // Old format
	FormatHeltec10Byte PayloadFormat = "heltec_10byte"  // Heltec TF02-Pro + DHT11
	FormatLilyGo13Byte PayloadFormat = "lilygo_13byte"  // LilyGo TF-Nova + Ultrasonic + DHT11
	FormatUnknown      PayloadFormat = "unknown"
)

// DecodedPayload - unified struct for all payload formats
type DecodedPayload struct {
	Format PayloadFormat

	// Common fields (all formats)
	SensorType     uint8  // 1 = TF02-Pro, flags for LilyGo, 0xFF = error
	DistanceMm     int16  // Primary distance in mm (-1 = error)
	SignalStrength int16  // Signal strength (flux)
	Temperature    int8   // Ambient temperature (legacy/LilyGo) or sensor temp (Heltec)
	BatteryPercent uint8  // Battery level (0-100)
	ReadingCount   uint8  // Number of valid readings

	// Heltec extended fields (10-byte format)
	SensorTemp  int8  // TF02-Pro internal temperature
	AmbientTemp int8  // DHT11 temperature
	Humidity    uint8 // DHT11 humidity %

	// LilyGo extended fields (13-byte format)
	SensorFlags       uint8  // Bit flags: bit0=TFNova, bit1=Ultrasonic, bit2=DHT, bit7=error
	UltrasonicDistMm  int16  // Second distance sensor in mm (-1 = error)
	BatteryMv         uint16 // Battery voltage in millivolts
}

// HandleMessage processes incoming MQTT messages from TTN
func HandleMessage(topic string, payload []byte) {
	if strings.HasSuffix(topic, "/join") {
		handleJoinMessage(payload)
		return
	}

	if strings.HasSuffix(topic, "/up") {
		handleUplinkMessage(payload)
		return
	}

	log.Printf("[MQTT] Unknown topic: %s", topic)
}

func handleJoinMessage(payload []byte) {
	var msg struct {
		EndDeviceIDs struct {
			DeviceID string `json:"device_id"`
			DevEUI   string `json:"dev_eui"`
			DevAddr  string `json:"dev_addr"`
		} `json:"end_device_ids"`
		ReceivedAt string `json:"received_at"`
	}

	if err := json.Unmarshal(payload, &msg); err != nil {
		log.Printf("[MQTT] Error parsing join message: %v", err)
		return
	}

	log.Printf("[JOIN] Device: %s (DevEUI: %s, DevAddr: %s) at %s",
		msg.EndDeviceIDs.DeviceID, msg.EndDeviceIDs.DevEUI, msg.EndDeviceIDs.DevAddr, msg.ReceivedAt)
}

func handleUplinkMessage(payload []byte) {
	var msg TTNUplinkMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		log.Printf("[MQTT] Error parsing uplink message: %v", err)
		return
	}

	deviceID := msg.EndDeviceIDs.DeviceID
	devEUI := msg.EndDeviceIDs.DevEUI

	log.Printf("[UPLINK] Device: %s (DevEUI: %s)", deviceID, devEUI)

	// Decode payload
	decoded := decodePayload(msg.UplinkMessage.FRMPayload)
	if decoded == nil {
		log.Printf("[UPLINK] Failed to decode payload for device %s", deviceID)
		return
	}

	// Get best gateway metadata
	bestGateway := msg.UplinkMessage.RXMetadata[0]
	if len(msg.UplinkMessage.RXMetadata) > 1 {
		for _, gw := range msg.UplinkMessage.RXMetadata {
			if gw.RSSI > bestGateway.RSSI {
				bestGateway = gw
			}
		}
	}

	// Ensure device exists
	device, err := ensureDevice(deviceID, devEUI)
	if err != nil {
		log.Printf("[UPLINK] Error ensuring device: %v", err)
		return
	}

	// Ensure signals exist for this device
	signals, err := ensureSignals(device.ID)
	if err != nil {
		log.Printf("[UPLINK] Error ensuring signals: %v", err)
		return
	}

	// Parse received_at timestamp
	receivedAt, err := time.Parse(time.RFC3339Nano, msg.ReceivedAt)
	if err != nil {
		receivedAt = time.Now()
	}

	// Store signal values
	storeSignalValues(device.ID, signals, decoded, receivedAt, &bestGateway, &msg)

	// Log based on format
	switch decoded.Format {
	case FormatHeltec10Byte:
		log.Printf("[UPLINK] Stored Heltec data for device %s: Distance=%dmm, SensorTemp=%d°C, AmbientTemp=%d°C, Humidity=%d%%, Battery=%d%%",
			deviceID, decoded.DistanceMm, decoded.SensorTemp, decoded.AmbientTemp, decoded.Humidity, decoded.BatteryPercent)
	case FormatLilyGo13Byte:
		log.Printf("[UPLINK] Stored LilyGo data for device %s: LiDAR=%dmm, Ultrasonic=%dmm, Temp=%d°C, Humidity=%d%%, Battery=%d%% (%dmV)",
			deviceID, decoded.DistanceMm, decoded.UltrasonicDistMm, decoded.Temperature, decoded.Humidity, decoded.BatteryPercent, decoded.BatteryMv)
	default:
		log.Printf("[UPLINK] Stored legacy data for device %s: Distance=%dmm, Battery=%d%%, Temp=%d°C",
			deviceID, decoded.DistanceMm, decoded.BatteryPercent, decoded.Temperature)
	}
}

func decodePayload(base64Payload string) *DecodedPayload {
	data, err := base64.StdEncoding.DecodeString(base64Payload)
	if err != nil {
		log.Printf("[Decoder] Failed to decode base64: %v", err)
		return nil
	}

	payloadLen := len(data)
	log.Printf("[Decoder] Payload length: %d bytes", payloadLen)

	if payloadLen < 8 {
		log.Printf("[Decoder] Payload too short: %d bytes (minimum 8)", payloadLen)
		return nil
	}

	// Detect format by payload length
	switch {
	case payloadLen >= 13:
		// LilyGo T-Beam format (13 bytes): TF-Nova + Ultrasonic + DHT11
		// Struct: sensorFlags(1), tfNovaDistMm(2), tfNovaStrength(2), ultrasonicDistMm(2),
		//         temperature(1), humidity(1), batteryPercent(1), batteryMv(2), readingCount(1)
		return &DecodedPayload{
			Format:           FormatLilyGo13Byte,
			SensorFlags:      uint8(data[0]),
			SensorType:       uint8(data[0]), // Use flags as sensor type for compatibility
			DistanceMm:       int16(binary.LittleEndian.Uint16(data[1:3])),
			SignalStrength:   int16(binary.LittleEndian.Uint16(data[3:5])),
			UltrasonicDistMm: int16(binary.LittleEndian.Uint16(data[5:7])),
			Temperature:      int8(data[7]),
			AmbientTemp:      int8(data[7]), // Same as temperature for LilyGo
			Humidity:         uint8(data[8]),
			BatteryPercent:   uint8(data[9]),
			BatteryMv:        binary.LittleEndian.Uint16(data[10:12]),
			ReadingCount:     uint8(data[12]),
		}

	case payloadLen >= 10:
		// Heltec TF02-Pro format (10 bytes): TF02-Pro + DHT11
		// Struct: sensorType(1), distanceMm(2), signalStrength(2), sensorTemp(1),
		//         ambientTemp(1), humidity(1), batteryPercent(1), readingCount(1)
		return &DecodedPayload{
			Format:         FormatHeltec10Byte,
			SensorType:     uint8(data[0]),
			DistanceMm:     int16(binary.LittleEndian.Uint16(data[1:3])),
			SignalStrength: int16(binary.LittleEndian.Uint16(data[3:5])),
			SensorTemp:     int8(data[5]),
			AmbientTemp:    int8(data[6]),
			Temperature:    int8(data[6]), // Use ambient temp as main temperature
			Humidity:       uint8(data[7]),
			BatteryPercent: uint8(data[8]),
			ReadingCount:   uint8(data[9]),
		}

	default:
		// Legacy 8-byte format
		// Struct: sensorType(1), distanceMm(2), signalStrength(2), temperature(1),
		//         batteryPercent(1), readingCount(1)
		return &DecodedPayload{
			Format:         FormatLegacy8Byte,
			SensorType:     uint8(data[0]),
			DistanceMm:     int16(binary.LittleEndian.Uint16(data[1:3])),
			SignalStrength: int16(binary.LittleEndian.Uint16(data[3:5])),
			Temperature:    int8(data[5]),
			AmbientTemp:    int8(data[5]), // Same as temperature for legacy
			BatteryPercent: uint8(data[6]),
			ReadingCount:   uint8(data[7]),
		}
	}
}

func ensureDevice(deviceID, devEUI string) (*models.Device, error) {
	database := db.GetDB()

	var device models.Device
	result := database.Where("name = ?", deviceID).First(&device)

	if result.Error != nil {
		// Device doesn't exist, create it
		device = models.Device{
			Name:        deviceID,
			Description: "TTN LoRaWAN Device",
			DeviceType:  "sensor",
			Location:    "TTN Network",
			AuthToken:   devEUI, // Use DevEUI as token for now
			IsActive:    true,
		}

		if err := database.Create(&device).Error; err != nil {
			return nil, err
		}

		log.Printf("[Device] Created device: %s (ID: %d)", deviceID, device.ID)
		return &device, nil
	}

	return &device, nil
}

// SignalConfig defines configuration for a signal type
type SignalConfig struct {
	Name        string
	SignalType  string
	Unit        string
	Description string
}

// All possible signals for TTN devices
var allSignalConfigs = []SignalConfig{
	// Common signals (all formats)
	{Name: "distance_mm", SignalType: "analogic", Unit: "mm", Description: "Primary distance (LiDAR)"},
	{Name: "battery_percent", SignalType: "analogic", Unit: "%", Description: "Battery level"},
	{Name: "temperature", SignalType: "analogic", Unit: "°C", Description: "Ambient temperature"},
	{Name: "signal_strength", SignalType: "analogic", Unit: "", Description: "LiDAR signal strength"},
	{Name: "reading_count", SignalType: "digital", Unit: "", Description: "Number of valid sensor readings"},

	// Extended signals (Heltec/LilyGo formats)
	{Name: "humidity", SignalType: "analogic", Unit: "%", Description: "Relative humidity (DHT11)"},
	{Name: "sensor_temp", SignalType: "analogic", Unit: "°C", Description: "Internal sensor temperature"},
	{Name: "ambient_temp", SignalType: "analogic", Unit: "°C", Description: "Ambient temperature (DHT11)"},

	// LilyGo-specific signals
	{Name: "ultrasonic_distance_mm", SignalType: "analogic", Unit: "mm", Description: "Ultrasonic distance"},
	{Name: "battery_mv", SignalType: "analogic", Unit: "mV", Description: "Battery voltage"},
	{Name: "sensor_flags", SignalType: "digital", Unit: "", Description: "Sensor status flags"},
}

func ensureSignals(deviceID uint) (map[string]uint, error) {
	database := db.GetDB()
	signals := make(map[string]uint)

	for _, cfg := range allSignalConfigs {
		var signal models.Signal
		result := database.Where("device_id = ? AND name = ?", deviceID, cfg.Name).First(&signal)

		if result.Error != nil {
			// Signal doesn't exist, create it
			signal = models.Signal{
				DeviceID:    deviceID,
				Name:        cfg.Name,
				SignalType:  cfg.SignalType,
				Direction:   "input",
				SensorName:  cfg.Name,
				Unit:        cfg.Unit,
				Description: cfg.Description,
				IsActive:    true,
			}

			if err := database.Create(&signal).Error; err != nil {
				return nil, err
			}

			log.Printf("[Signal] Created signal: %s (ID: %d) for device %d", cfg.Name, signal.ID, deviceID)
		}

		signals[cfg.Name] = signal.ID
	}

	return signals, nil
}

func storeSignalValues(deviceID uint, signals map[string]uint, decoded *DecodedPayload, timestamp time.Time, gateway *struct {
	GatewayIDs struct {
		GatewayID string `json:"gateway_id"`
	} `json:"gateway_ids"`
	RSSI int     `json:"rssi"`
	SNR  float64 `json:"snr"`
}, msg *TTNUplinkMessage) {
	database := db.GetDB()

	// Common metadata for primary distance signal
	distanceMetadata := models.JSONB{
		"gateway_id":     gateway.GatewayIDs.GatewayID,
		"rssi":           gateway.RSSI,
		"snr":            gateway.SNR,
		"f_cnt":          msg.UplinkMessage.FCnt,
		"payload_format": string(decoded.Format),
	}

	// Build list of values to store based on payload format
	type signalEntry struct {
		SignalID uint
		Value    *float64
		Metadata models.JSONB
	}

	var values []signalEntry

	// Common signals (all formats)
	values = append(values,
		signalEntry{
			SignalID: signals["distance_mm"],
			Value:    floatPtr(float64(decoded.DistanceMm)),
			Metadata: distanceMetadata,
		},
		signalEntry{
			SignalID: signals["battery_percent"],
			Value:    floatPtr(float64(decoded.BatteryPercent)),
		},
		signalEntry{
			SignalID: signals["signal_strength"],
			Value:    floatPtr(float64(decoded.SignalStrength)),
		},
		signalEntry{
			SignalID: signals["reading_count"],
			Value:    floatPtr(float64(decoded.ReadingCount)),
		},
	)

	// Format-specific signals
	switch decoded.Format {
	case FormatHeltec10Byte:
		// Heltec: has sensor_temp, ambient_temp, humidity
		values = append(values,
			signalEntry{
				SignalID: signals["sensor_temp"],
				Value:    floatPtr(float64(decoded.SensorTemp)),
			},
			signalEntry{
				SignalID: signals["ambient_temp"],
				Value:    floatPtr(float64(decoded.AmbientTemp)),
			},
			signalEntry{
				SignalID: signals["temperature"],
				Value:    floatPtr(float64(decoded.AmbientTemp)), // Use ambient as main temp
			},
			signalEntry{
				SignalID: signals["humidity"],
				Value:    floatPtr(float64(decoded.Humidity)),
			},
		)

	case FormatLilyGo13Byte:
		// LilyGo: has ultrasonic, humidity, battery_mv, sensor_flags
		values = append(values,
			signalEntry{
				SignalID: signals["temperature"],
				Value:    floatPtr(float64(decoded.Temperature)),
			},
			signalEntry{
				SignalID: signals["ambient_temp"],
				Value:    floatPtr(float64(decoded.AmbientTemp)),
			},
			signalEntry{
				SignalID: signals["humidity"],
				Value:    floatPtr(float64(decoded.Humidity)),
			},
			signalEntry{
				SignalID: signals["ultrasonic_distance_mm"],
				Value:    floatPtr(float64(decoded.UltrasonicDistMm)),
			},
			signalEntry{
				SignalID: signals["battery_mv"],
				Value:    floatPtr(float64(decoded.BatteryMv)),
			},
			signalEntry{
				SignalID: signals["sensor_flags"],
				Value:    floatPtr(float64(decoded.SensorFlags)),
			},
		)

	default:
		// Legacy format: just temperature
		values = append(values,
			signalEntry{
				SignalID: signals["temperature"],
				Value:    floatPtr(float64(decoded.Temperature)),
			},
		)
	}

	// Store all values
	for _, v := range values {
		signalValue := models.SignalValue{
			SignalID:  v.SignalID,
			Timestamp: timestamp,
			Value:     v.Value,
			Metadata:  v.Metadata,
		}

		if err := database.Create(&signalValue).Error; err != nil {
			log.Printf("[DB] Error storing signal value: %v", err)
		}
	}
}

func floatPtr(f float64) *float64 {
	return &f
}
