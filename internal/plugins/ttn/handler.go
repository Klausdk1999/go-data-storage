package ttn

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"data-storage/internal/db"
	"data-storage/internal/models"
)

func handleMessage(topic string, payload []byte) {
	if strings.HasSuffix(topic, "/join") {
		handleJoinMessage(payload)
		return
	}

	if strings.HasSuffix(topic, "/up") {
		handleUplinkMessage(payload)
		return
	}

	log.Printf("[TTN Plugin] Unknown topic: %s", topic)
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
		log.Printf("[TTN Plugin] Error parsing join message: %v", err)
		return
	}

	log.Printf("[TTN Plugin] JOIN Device: %s (DevEUI: %s, DevAddr: %s) at %s",
		msg.EndDeviceIDs.DeviceID, msg.EndDeviceIDs.DevEUI, msg.EndDeviceIDs.DevAddr, msg.ReceivedAt)
}

func handleUplinkMessage(payload []byte) {
	var msg TTNUplinkMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		log.Printf("[TTN Plugin] Error parsing uplink message: %v", err)
		return
	}

	deviceID := msg.EndDeviceIDs.DeviceID
	devEUI := msg.EndDeviceIDs.DevEUI

	log.Printf("[TTN Plugin] UPLINK Device: %s (DevEUI: %s)", deviceID, devEUI)

	// Decode payload
	decoded := decodePayload(msg.UplinkMessage.FRMPayload)
	if decoded == nil {
		log.Printf("[TTN Plugin] Failed to decode payload for device %s", deviceID)
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
		log.Printf("[TTN Plugin] Error ensuring device: %v", err)
		return
	}

	// Ensure signals exist for this device
	signals, err := ensureSignals(device.ID)
	if err != nil {
		log.Printf("[TTN Plugin] Error ensuring signals: %v", err)
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
		log.Printf("[TTN Plugin] Stored Heltec data for device %s: Distance=%dmm, SensorTemp=%d°C, AmbientTemp=%d°C, Humidity=%d%%, Battery=%d%%",
			deviceID, decoded.DistanceMm, decoded.SensorTemp, decoded.AmbientTemp, decoded.Humidity, decoded.BatteryPercent)
	case FormatLilyGo13Byte:
		log.Printf("[TTN Plugin] Stored LilyGo data for device %s: LiDAR=%dmm, Ultrasonic=%dmm, Temp=%d°C, Humidity=%d%%, Battery=%d%% (%dmV)",
			deviceID, decoded.DistanceMm, decoded.UltrasonicDistMm, decoded.Temperature, decoded.Humidity, decoded.BatteryPercent, decoded.BatteryMv)
	default:
		log.Printf("[TTN Plugin] Stored legacy data for device %s: Distance=%dmm, Battery=%d%%, Temp=%d°C",
			deviceID, decoded.DistanceMm, decoded.BatteryPercent, decoded.Temperature)
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
			AuthToken:   devEUI,
			IsActive:    true,
		}

		if err := database.Create(&device).Error; err != nil {
			return nil, err
		}

		log.Printf("[TTN Plugin] Created device: %s (ID: %d)", deviceID, device.ID)
		return &device, nil
	}

	return &device, nil
}

// signalConfig defines configuration for a signal type.
type signalConfig struct {
	Name        string
	SignalType  string
	Unit        string
	Description string
}

// All possible signals for TTN devices.
var allSignalConfigs = []signalConfig{
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

			log.Printf("[TTN Plugin] Created signal: %s (ID: %d) for device %d", cfg.Name, signal.ID, deviceID)
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

	type signalEntry struct {
		SignalID  uint
		Value    *float64
		Metadata models.JSONB
	}

	var values []signalEntry

	// Common signals (all formats)
	values = append(values,
		signalEntry{
			SignalID:  signals["distance_mm"],
			Value:    floatPtr(float64(decoded.DistanceMm)),
			Metadata: distanceMetadata,
		},
		signalEntry{
			SignalID: signals["battery_percent"],
			Value:   floatPtr(float64(decoded.BatteryPercent)),
		},
		signalEntry{
			SignalID: signals["signal_strength"],
			Value:   floatPtr(float64(decoded.SignalStrength)),
		},
		signalEntry{
			SignalID: signals["reading_count"],
			Value:   floatPtr(float64(decoded.ReadingCount)),
		},
	)

	// Format-specific signals
	switch decoded.Format {
	case FormatHeltec10Byte:
		values = append(values,
			signalEntry{
				SignalID: signals["sensor_temp"],
				Value:   floatPtr(float64(decoded.SensorTemp)),
			},
			signalEntry{
				SignalID: signals["ambient_temp"],
				Value:   floatPtr(float64(decoded.AmbientTemp)),
			},
			signalEntry{
				SignalID: signals["temperature"],
				Value:   floatPtr(float64(decoded.AmbientTemp)),
			},
			signalEntry{
				SignalID: signals["humidity"],
				Value:   floatPtr(float64(decoded.Humidity)),
			},
		)

	case FormatLilyGo13Byte:
		values = append(values,
			signalEntry{
				SignalID: signals["temperature"],
				Value:   floatPtr(float64(decoded.Temperature)),
			},
			signalEntry{
				SignalID: signals["ambient_temp"],
				Value:   floatPtr(float64(decoded.AmbientTemp)),
			},
			signalEntry{
				SignalID: signals["humidity"],
				Value:   floatPtr(float64(decoded.Humidity)),
			},
			signalEntry{
				SignalID: signals["ultrasonic_distance_mm"],
				Value:   floatPtr(float64(decoded.UltrasonicDistMm)),
			},
			signalEntry{
				SignalID: signals["battery_mv"],
				Value:   floatPtr(float64(decoded.BatteryMv)),
			},
			signalEntry{
				SignalID: signals["sensor_flags"],
				Value:   floatPtr(float64(decoded.SensorFlags)),
			},
		)

	default:
		values = append(values,
			signalEntry{
				SignalID: signals["temperature"],
				Value:   floatPtr(float64(decoded.Temperature)),
			},
		)
	}

	// Store all values
	for _, v := range values {
		signalValue := models.SignalValue{
			SignalID:   v.SignalID,
			Timestamp: timestamp,
			Value:     v.Value,
			Metadata:  v.Metadata,
		}

		if err := database.Create(&signalValue).Error; err != nil {
			log.Printf("[TTN Plugin] Error storing signal value: %v", err)
		}
	}
}

func floatPtr(f float64) *float64 {
	return &f
}
