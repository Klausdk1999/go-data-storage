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

// DecodedPayload matches the SensorPayload struct from firmware
type DecodedPayload struct {
	SensorType     uint8
	DistanceMm     uint16
	SignalStrength int16
	Temperature    int8
	BatteryPercent uint8
	ReadingCount   uint8
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

	log.Printf("[UPLINK] Stored data for device %s: Distance=%dmm, Battery=%d%%, Temp=%d°C",
		deviceID, decoded.DistanceMm, decoded.BatteryPercent, decoded.Temperature)
}

func decodePayload(base64Payload string) *DecodedPayload {
	data, err := base64.StdEncoding.DecodeString(base64Payload)
	if err != nil {
		log.Printf("[Decoder] Failed to decode base64: %v", err)
		return nil
	}

	if len(data) < 8 {
		log.Printf("[Decoder] Payload too short: %d bytes (expected 8)", len(data))
		return nil
	}

	return &DecodedPayload{
		SensorType:     uint8(data[0]),
		DistanceMm:     binary.LittleEndian.Uint16(data[1:3]),
		SignalStrength: int16(binary.LittleEndian.Uint16(data[3:5])),
		Temperature:    int8(data[5]),
		BatteryPercent: uint8(data[6]),
		ReadingCount:   uint8(data[7]),
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

func ensureSignals(deviceID uint) (map[string]uint, error) {
	database := db.GetDB()
	signals := make(map[string]uint)

	signalNames := []string{"distance_mm", "battery_percent", "temperature", "signal_strength", "reading_count"}

	for _, name := range signalNames {
		var signal models.Signal
		result := database.Where("device_id = ? AND name = ?", deviceID, name).First(&signal)

		if result.Error != nil {
			// Signal doesn't exist, create it
			signalType := "analogic"
			unit := ""
			if name == "battery_percent" {
				unit = "%"
			} else if name == "distance_mm" {
				unit = "mm"
			} else if name == "temperature" {
				unit = "°C"
			} else if name == "reading_count" {
				signalType = "digital"
			}

			signal = models.Signal{
				DeviceID:   deviceID,
				Name:       name,
				SignalType: signalType,
				Direction:  "input",
				SensorName: name,
				Unit:       unit,
				IsActive:   true,
			}

			if err := database.Create(&signal).Error; err != nil {
				return nil, err
			}

			log.Printf("[Signal] Created signal: %s (ID: %d) for device %d", name, signal.ID, deviceID)
		}

		signals[name] = signal.ID
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

	// Store each value
	values := []struct {
		SignalID uint
		Value    *float64
		Metadata models.JSONB
	}{
		{
			SignalID: signals["distance_mm"],
			Value:    floatPtr(float64(decoded.DistanceMm)),
			Metadata: models.JSONB{
				"gateway_id": gateway.GatewayIDs.GatewayID,
				"rssi":       gateway.RSSI,
				"snr":        gateway.SNR,
				"f_cnt":      msg.UplinkMessage.FCnt,
			},
		},
		{
			SignalID: signals["battery_percent"],
			Value:    floatPtr(float64(decoded.BatteryPercent)),
		},
		{
			SignalID: signals["temperature"],
			Value:    floatPtr(float64(decoded.Temperature)),
		},
		{
			SignalID: signals["signal_strength"],
			Value:    floatPtr(float64(decoded.SignalStrength)),
		},
	}

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
