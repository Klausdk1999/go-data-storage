package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"data-storage/internal/db"
	"data-storage/internal/models"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// SignalValuesHandler handles signal value CRUD operations
func SignalValuesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllSignalValues(w, r)
	case "POST":
		CreateSignalValue(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// SignalValueHandler handles individual signal value operations
func SignalValueHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getSignalValue(w, r)
	case "DELETE":
		deleteSignalValue(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getAllSignalValues(w http.ResponseWriter, r *http.Request) {
	var signalValues []models.SignalValue
	query := db.GetDB().Preload("Signal").Preload("Signal.Device").Preload("User")

	// Filter by signal_id
	if signalID := r.URL.Query().Get("signal_id"); signalID != "" {
		query = query.Where("signal_id = ?", signalID)
	}

	// Filter by device_id (through signal)
	if deviceID := r.URL.Query().Get("device_id"); deviceID != "" {
		query = query.Joins("JOIN signals ON signal_values.signal_id = signals.id").
			Where("signals.device_id = ?", deviceID)
	}

	// Filter by user_id
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	// Date range filters
	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		query = query.Where("timestamp >= ?", fromDate)
	}
	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		query = query.Where("timestamp <= ?", toDate)
	}

	// Limit results
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "1000" // Default limit
	}
	limitInt, _ := strconv.Atoi(limit)
	if limitInt > 10000 {
		limitInt = 10000 // Max limit
	}

	result := query.Order("timestamp DESC").Limit(limitInt).Find(&signalValues)
	if result.Error != nil {
		log.Printf("Error fetching signal values: %v", result.Error)
		http.Error(w, "Error fetching signal values", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signalValues)
}

func getSignalValue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	valueIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Signal value ID required", http.StatusBadRequest)
		return
	}

	valueID, err := strconv.ParseUint(valueIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid signal value ID", http.StatusBadRequest)
		return
	}

	var signalValue models.SignalValue
	result := db.GetDB().Preload("Signal").Preload("Signal.Device").Preload("User").First(&signalValue, valueID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Signal value not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching signal value: %v", result.Error)
			http.Error(w, "Error fetching signal value", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signalValue)
}

// CreateSignalValue is exported for use in main.go routing
func CreateSignalValue(w http.ResponseWriter, r *http.Request) {
	var signalValue models.SignalValue
	if err := json.NewDecoder(r.Body).Decode(&signalValue); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Signal ID is required
	if signalValue.SignalID == 0 {
		http.Error(w, "signal_id is required", http.StatusBadRequest)
		return
	}

	// Verify signal exists and get device info
	var signal models.Signal
	result := db.GetDB().Preload("Device").First(&signal, signalValue.SignalID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Signal not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching signal: %v", result.Error)
			http.Error(w, "Error fetching signal", http.StatusInternalServerError)
		}
		return
	}

	// Determine user_id: use provided, fallback to device user, or none
	if signalValue.UserID == nil && signal.Device.UserID != nil {
		signalValue.UserID = signal.Device.UserID
	}

	// Get auth info from headers (set by middleware)
	authType := r.Header.Get("X-Auth-Type")
	deviceIDStr := r.Header.Get("X-Device-ID")
	userIDStr := r.Header.Get("X-User-ID")

	// If authenticated via device token, ensure device_id matches
	if authType == "device" && deviceIDStr != "" {
		authDeviceID, _ := strconv.ParseUint(deviceIDStr, 10, 32)
		if uint(authDeviceID) != signal.DeviceID {
			http.Error(w, "Device ID mismatch", http.StatusForbidden)
			return
		}
	}

	// If authenticated via user, allow setting user_id or use authenticated user
	if authType == "user" && userIDStr != "" {
		authUserID, _ := strconv.ParseUint(userIDStr, 10, 32)
		// If no user_id provided, use authenticated user
		if signalValue.UserID == nil {
			authUID := uint(authUserID)
			signalValue.UserID = &authUID
		}
	}

	// Validate value based on signal type
	if signal.SignalType == "analogic" {
		if signalValue.Value == nil {
			http.Error(w, "value is required for analogic signals", http.StatusBadRequest)
			return
		}
		// Validate min/max if set
		if signal.MinValue != nil && *signalValue.Value < *signal.MinValue {
			http.Error(w, "value below minimum", http.StatusBadRequest)
			return
		}
		if signal.MaxValue != nil && *signalValue.Value > *signal.MaxValue {
			http.Error(w, "value above maximum", http.StatusBadRequest)
			return
		}
	} else if signal.SignalType == "digital" {
		if signalValue.DigitalValue == nil {
			http.Error(w, "digital_value is required for digital signals", http.StatusBadRequest)
			return
		}
	}

	// Set timestamp if not provided
	if signalValue.Timestamp.IsZero() {
		signalValue.Timestamp = time.Now()
	}

	result = db.GetDB().Create(&signalValue)
	if result.Error != nil {
		log.Printf("Error creating signal value: %v", result.Error)
		http.Error(w, "Error creating signal value", http.StatusInternalServerError)
		return
	}

	// Reload with relations
	db.GetDB().Preload("Signal").Preload("Signal.Device").Preload("User").First(&signalValue, signalValue.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(signalValue)
}

func deleteSignalValue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	valueIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Signal value ID required", http.StatusBadRequest)
		return
	}

	valueID, err := strconv.ParseUint(valueIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid signal value ID", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Delete(&models.SignalValue{}, valueID)
	if result.Error != nil {
		log.Printf("Error deleting signal value: %v", result.Error)
		http.Error(w, "Error deleting signal value", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "Signal value not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SignalValuesBySignalHandler gets signal values for a specific signal
func SignalValuesBySignalHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	signalIDStr, ok := vars["signal_id"]
	if !ok {
		http.Error(w, "Signal ID required", http.StatusBadRequest)
		return
	}

	signalID, err := strconv.ParseUint(signalIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid signal ID", http.StatusBadRequest)
		return
	}

	var signalValues []models.SignalValue
	query := db.GetDB().Where("signal_id = ?", signalID).Preload("Signal").Preload("User")

	// Date range filters
	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		query = query.Where("timestamp >= ?", fromDate)
	}
	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		query = query.Where("timestamp <= ?", toDate)
	}

	// Limit
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "1000"
	}
	limitInt, _ := strconv.Atoi(limit)
	if limitInt > 10000 {
		limitInt = 10000
	}

	result := query.Order("timestamp DESC").Limit(limitInt).Find(&signalValues)
	if result.Error != nil {
		log.Printf("Error fetching signal values: %v", result.Error)
		http.Error(w, "Error fetching signal values", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signalValues)
}

