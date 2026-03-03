package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"data-storage/internal/auth"
	"data-storage/internal/db"
	"data-storage/internal/models"

	"gorm.io/gorm"
)

// GenericDeviceDataRaw uses interface{} for flexible field parsing from any microcontroller.
type GenericDeviceDataRaw struct {
	DeviceID string      `json:"device_id"`
	Field1   interface{} `json:"field_1"`
	Field2   interface{} `json:"field_2"`
	Field3   interface{} `json:"field_3"`
	Field4   interface{} `json:"field_4"`
	Field5   interface{} `json:"field_5"`
	Field6   interface{} `json:"field_6"`
	Field7   interface{} `json:"field_7"`
	Field8   interface{} `json:"field_8"`
}

// GenericDataHandler handles POST /devices/data.
// It accepts authentication via X-API-Key header (matched against DEVICE_API_KEY env)
// or via Authorization: Bearer <device_token>.
func GenericDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// --- Authentication ---
	var deviceToken string
	authenticated := false

	// Check X-API-Key header against DEVICE_API_KEY env var
	apiKey := r.Header.Get("X-API-Key")
	envAPIKey := os.Getenv("DEVICE_API_KEY")
	if apiKey != "" && envAPIKey != "" && apiKey == envAPIKey {
		authenticated = true
	}

	// Check Authorization: Bearer <device_token>
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			deviceToken = parts[1]
			// Verify the token exists in the database
			_, err := auth.AuthenticateDevice(deviceToken)
			if err == nil {
				authenticated = true
			}
		}
	}

	if !authenticated {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// --- Parse body ---
	var data GenericDeviceDataRaw
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if data.DeviceID == "" {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}

	// --- Process ---
	if err := ProcessGenericDeviceData(data, deviceToken); err != nil {
		log.Printf("Error processing generic device data: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ProcessGenericDeviceData finds or creates the device and stores signal values.
// It is exported so it can be reused by the MQTT broker handler.
func ProcessGenericDeviceData(data GenericDeviceDataRaw, deviceToken string) error {
	database := db.GetDB()

	// Find device by name
	var device models.Device
	result := database.Where("name = ?", data.DeviceID).First(&device)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Auto-create device unless disabled
			autoCreate := os.Getenv("DEVICE_AUTO_CREATE")
			if autoCreate == "false" {
				return fmt.Errorf("device %q not found and auto-create is disabled", data.DeviceID)
			}

			token, err := auth.GenerateDeviceToken()
			if err != nil {
				return fmt.Errorf("failed to generate device token: %w", err)
			}

			device = models.Device{
				Name:       data.DeviceID,
				DeviceType: "generic",
				AuthToken:  token,
				IsActive:   true,
			}
			if err := database.Create(&device).Error; err != nil {
				return fmt.Errorf("failed to auto-create device: %w", err)
			}
			log.Printf("Auto-created device %q (ID=%d)", device.Name, device.ID)
		} else {
			return fmt.Errorf("failed to look up device: %w", result.Error)
		}
	}

	// If a device token was provided, verify it matches the device's auth_token
	if deviceToken != "" && device.AuthToken != deviceToken {
		return fmt.Errorf("device token mismatch for device %q", data.DeviceID)
	}

	// Process each non-nil field
	fields := map[string]interface{}{
		"field_1": data.Field1,
		"field_2": data.Field2,
		"field_3": data.Field3,
		"field_4": data.Field4,
		"field_5": data.Field5,
		"field_6": data.Field6,
		"field_7": data.Field7,
		"field_8": data.Field8,
	}

	now := time.Now()

	for fieldName, rawValue := range fields {
		if rawValue == nil {
			continue
		}

		signal, err := ensureGenericSignal(database, device.ID, fieldName, rawValue)
		if err != nil {
			return fmt.Errorf("failed to ensure signal %s: %w", fieldName, err)
		}

		sv := models.SignalValue{
			SignalID:   signal.ID,
			UserID:     device.UserID,
			Timestamp:  now,
		}

		switch v := rawValue.(type) {
		case float64:
			sv.Value = &v
		case bool:
			sv.DigitalValue = &v
		case string:
			zero := 0.0
			sv.Value = &zero
			sv.Metadata = models.JSONB{"string_value": v}
		default:
			// JSON numbers are decoded as float64 by encoding/json.
			// For any other type, attempt to store as string in metadata.
			zero := 0.0
			sv.Value = &zero
			sv.Metadata = models.JSONB{"string_value": fmt.Sprintf("%v", v)}
		}

		if err := database.Create(&sv).Error; err != nil {
			return fmt.Errorf("failed to store signal value for %s: %w", fieldName, err)
		}
	}

	return nil
}

// ensureGenericSignal finds or creates a signal for the given device and field name.
func ensureGenericSignal(database *gorm.DB, deviceID uint, fieldName string, value interface{}) (*models.Signal, error) {
	var signal models.Signal
	result := database.Where("device_id = ? AND name = ?", deviceID, fieldName).First(&signal)

	if result.Error == nil {
		return &signal, nil
	}

	if result.Error != gorm.ErrRecordNotFound {
		return nil, result.Error
	}

	// Determine signal type from value type
	signalType := "analogic"
	if _, ok := value.(bool); ok {
		signalType = "digital"
	}

	signal = models.Signal{
		DeviceID:   deviceID,
		Name:       fieldName,
		SignalType: signalType,
		Direction:  "input",
		IsActive:   true,
	}

	if err := database.Create(&signal).Error; err != nil {
		return nil, err
	}

	log.Printf("Auto-created signal %q (ID=%d) for device ID=%d", fieldName, signal.ID, deviceID)
	return &signal, nil
}
