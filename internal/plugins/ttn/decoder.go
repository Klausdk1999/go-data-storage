package ttn

import (
	"encoding/base64"
	"encoding/binary"
	"log"
)

// TTNUplinkMessage matches the structure from TTN MQTT messages.
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

// PayloadFormat identifies which device format was detected.
type PayloadFormat string

const (
	FormatLegacy8Byte  PayloadFormat = "legacy_8byte"  // Old format
	FormatHeltec10Byte PayloadFormat = "heltec_10byte" // Heltec TF02-Pro + DHT11
	FormatLilyGo13Byte PayloadFormat = "lilygo_13byte" // LilyGo TF-Nova + Ultrasonic + DHT11
	FormatUnknown      PayloadFormat = "unknown"
)

// DecodedPayload is a unified struct for all payload formats.
type DecodedPayload struct {
	Format PayloadFormat

	// Common fields (all formats)
	SensorType     uint8 // 1 = TF02-Pro, flags for LilyGo, 0xFF = error
	DistanceMm     int16 // Primary distance in mm (-1 = error)
	SignalStrength int16 // Signal strength (flux)
	Temperature    int8  // Ambient temperature (legacy/LilyGo) or sensor temp (Heltec)
	BatteryPercent uint8 // Battery level (0-100)
	ReadingCount   uint8 // Number of valid readings

	// Heltec extended fields (10-byte format)
	SensorTemp  int8  // TF02-Pro internal temperature
	AmbientTemp int8  // DHT11 temperature
	Humidity    uint8 // DHT11 humidity %

	// LilyGo extended fields (13-byte format)
	SensorFlags      uint8  // Bit flags: bit0=TFNova, bit1=Ultrasonic, bit2=DHT, bit7=error
	UltrasonicDistMm int16  // Second distance sensor in mm (-1 = error)
	BatteryMv        uint16 // Battery voltage in millivolts
}

func decodePayload(base64Payload string) *DecodedPayload {
	data, err := base64.StdEncoding.DecodeString(base64Payload)
	if err != nil {
		log.Printf("[TTN Plugin] Failed to decode base64: %v", err)
		return nil
	}

	payloadLen := len(data)
	log.Printf("[TTN Plugin] Payload length: %d bytes", payloadLen)

	if payloadLen < 8 {
		log.Printf("[TTN Plugin] Payload too short: %d bytes (minimum 8)", payloadLen)
		return nil
	}

	// Detect format by payload length
	switch {
	case payloadLen >= 13:
		// LilyGo T-Beam format (13 bytes): TF-Nova + Ultrasonic + DHT11
		return &DecodedPayload{
			Format:           FormatLilyGo13Byte,
			SensorFlags:      uint8(data[0]),
			SensorType:       uint8(data[0]),
			DistanceMm:       int16(binary.LittleEndian.Uint16(data[1:3])),
			SignalStrength:   int16(binary.LittleEndian.Uint16(data[3:5])),
			UltrasonicDistMm: int16(binary.LittleEndian.Uint16(data[5:7])),
			Temperature:      int8(data[7]),
			AmbientTemp:      int8(data[7]),
			Humidity:         uint8(data[8]),
			BatteryPercent:   uint8(data[9]),
			BatteryMv:        binary.LittleEndian.Uint16(data[10:12]),
			ReadingCount:     uint8(data[12]),
		}

	case payloadLen >= 10:
		// Heltec TF02-Pro format (10 bytes): TF02-Pro + DHT11
		return &DecodedPayload{
			Format:         FormatHeltec10Byte,
			SensorType:     uint8(data[0]),
			DistanceMm:     int16(binary.LittleEndian.Uint16(data[1:3])),
			SignalStrength: int16(binary.LittleEndian.Uint16(data[3:5])),
			SensorTemp:     int8(data[5]),
			AmbientTemp:    int8(data[6]),
			Temperature:    int8(data[6]),
			Humidity:       uint8(data[7]),
			BatteryPercent: uint8(data[8]),
			ReadingCount:   uint8(data[9]),
		}

	default:
		// Legacy 8-byte format
		return &DecodedPayload{
			Format:         FormatLegacy8Byte,
			SensorType:     uint8(data[0]),
			DistanceMm:     int16(binary.LittleEndian.Uint16(data[1:3])),
			SignalStrength: int16(binary.LittleEndian.Uint16(data[3:5])),
			Temperature:    int8(data[5]),
			AmbientTemp:    int8(data[5]),
			BatteryPercent: uint8(data[6]),
			ReadingCount:   uint8(data[7]),
		}
	}
}
