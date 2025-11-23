package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"data-storage/internal/db"
	"data-storage/internal/models"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// SignalsHandler handles signal configuration CRUD operations
func SignalsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllSignals(w, r)
	case "POST":
		createSignal(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// SignalHandler handles individual signal configuration operations
func SignalHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getSignal(w, r)
	case "PUT":
		updateSignal(w, r)
	case "DELETE":
		deleteSignal(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getAllSignals(w http.ResponseWriter, r *http.Request) {
	var signals []models.Signal
	query := db.GetDB().Preload("Device")

	// Filter by device_id
	if deviceID := r.URL.Query().Get("device_id"); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}

	// Filter by signal_type
	if signalType := r.URL.Query().Get("signal_type"); signalType != "" {
		query = query.Where("signal_type = ?", signalType)
	}

	// Filter by direction
	if direction := r.URL.Query().Get("direction"); direction != "" {
		query = query.Where("direction = ?", direction)
	}

	// Filter by active status
	if active := r.URL.Query().Get("active"); active != "" {
		isActive := active == "true"
		query = query.Where("is_active = ?", isActive)
	}

	result := query.Order("created_at DESC").Find(&signals)
	if result.Error != nil {
		log.Printf("Error fetching signals: %v", result.Error)
		http.Error(w, "Error fetching signals", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signals)
}

func getSignal(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	signalIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Signal ID required", http.StatusBadRequest)
		return
	}

	signalID, err := strconv.ParseUint(signalIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid signal ID", http.StatusBadRequest)
		return
	}

	var signal models.Signal
	result := db.GetDB().Preload("Device").First(&signal, signalID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Signal not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching signal: %v", result.Error)
			http.Error(w, "Error fetching signal", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signal)
}

func createSignal(w http.ResponseWriter, r *http.Request) {
	var signal models.Signal
	if err := json.NewDecoder(r.Body).Decode(&signal); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Device ID is required
	if signal.DeviceID == 0 {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}

	// Verify device exists
	var device models.Device
	result := db.GetDB().First(&device, signal.DeviceID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Device not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching device: %v", result.Error)
			http.Error(w, "Error fetching device", http.StatusInternalServerError)
		}
		return
	}

	// Set defaults
	if signal.SignalType == "" {
		signal.SignalType = "analogic"
	}
	if signal.Direction == "" {
		signal.Direction = "input"
	}
	if signal.IsActive == false && signal.ID == 0 {
		signal.IsActive = true
	}

	// Validate signal_type
	if signal.SignalType != "digital" && signal.SignalType != "analogic" {
		http.Error(w, "signal_type must be 'digital' or 'analogic'", http.StatusBadRequest)
		return
	}

	// Validate direction
	if signal.Direction != "input" && signal.Direction != "output" {
		http.Error(w, "direction must be 'input' or 'output'", http.StatusBadRequest)
		return
	}

	result = db.GetDB().Create(&signal)
	if result.Error != nil {
		log.Printf("Error creating signal: %v", result.Error)
		http.Error(w, "Error creating signal", http.StatusInternalServerError)
		return
	}

	// Reload with relations
	db.GetDB().Preload("Device").First(&signal, signal.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(signal)
}

func updateSignal(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	signalIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Signal ID required", http.StatusBadRequest)
		return
	}

	signalID, err := strconv.ParseUint(signalIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid signal ID", http.StatusBadRequest)
		return
	}

	var signal models.Signal
	result := db.GetDB().First(&signal, signalID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Signal not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching signal", http.StatusInternalServerError)
		}
		return
	}

	var updateData models.Signal
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update fields
	if updateData.Name != "" {
		signal.Name = updateData.Name
	}
	if updateData.SignalType != "" {
		if updateData.SignalType != "digital" && updateData.SignalType != "analogic" {
			http.Error(w, "signal_type must be 'digital' or 'analogic'", http.StatusBadRequest)
			return
		}
		signal.SignalType = updateData.SignalType
	}
	if updateData.Direction != "" {
		if updateData.Direction != "input" && updateData.Direction != "output" {
			http.Error(w, "direction must be 'input' or 'output'", http.StatusBadRequest)
			return
		}
		signal.Direction = updateData.Direction
	}
	if updateData.SensorName != "" {
		signal.SensorName = updateData.SensorName
	}
	if updateData.Description != "" {
		signal.Description = updateData.Description
	}
	if updateData.Unit != "" {
		signal.Unit = updateData.Unit
	}
	if updateData.MinValue != nil {
		signal.MinValue = updateData.MinValue
	}
	if updateData.MaxValue != nil {
		signal.MaxValue = updateData.MaxValue
	}
	if updateData.Metadata != nil {
		signal.Metadata = updateData.Metadata
	}
	// is_active can be explicitly set
	signal.IsActive = updateData.IsActive

	result = db.GetDB().Save(&signal)
	if result.Error != nil {
		log.Printf("Error updating signal: %v", result.Error)
		http.Error(w, "Error updating signal", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signal)
}

func deleteSignal(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	signalIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Signal ID required", http.StatusBadRequest)
		return
	}

	signalID, err := strconv.ParseUint(signalIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid signal ID", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Delete(&models.Signal{}, signalID)
	if result.Error != nil {
		log.Printf("Error deleting signal: %v", result.Error)
		http.Error(w, "Error deleting signal", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "Signal not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeviceSignalsHandler gets signal configurations for a specific device
func DeviceSignalsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	deviceIDStr, ok := vars["device_id"]
	if !ok {
		http.Error(w, "Device ID required", http.StatusBadRequest)
		return
	}

	deviceID, err := strconv.ParseUint(deviceIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	var signals []models.Signal
	query := db.GetDB().Where("device_id = ?", deviceID).Preload("Device")

	// Apply filters
	if signalType := r.URL.Query().Get("signal_type"); signalType != "" {
		query = query.Where("signal_type = ?", signalType)
	}
	if direction := r.URL.Query().Get("direction"); direction != "" {
		query = query.Where("direction = ?", direction)
	}

	result := query.Order("created_at DESC").Find(&signals)
	if result.Error != nil {
		log.Printf("Error fetching device signals: %v", result.Error)
		http.Error(w, "Error fetching device signals", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signals)
}
