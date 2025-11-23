package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"data-storage/internal/auth"
	"data-storage/internal/db"
	"data-storage/internal/models"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// DevicesHandler handles device CRUD operations
func DevicesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllDevices(w, r)
	case "POST":
		createDevice(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// DeviceHandler handles individual device operations
func DeviceHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getDevice(w, r)
	case "PUT":
		updateDevice(w, r)
	case "DELETE":
		deleteDevice(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getAllDevices(w http.ResponseWriter, r *http.Request) {
	var devices []models.Device
	query := db.GetDB().Preload("User")

	// Filter by user_id if provided
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	// Filter by active status
	if active := r.URL.Query().Get("active"); active != "" {
		isActive := active == "true"
		query = query.Where("is_active = ?", isActive)
	}

	result := query.Find(&devices)
	if result.Error != nil {
		log.Printf("Error fetching devices: %v", result.Error)
		http.Error(w, "Error fetching devices", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func getDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Device ID required", http.StatusBadRequest)
		return
	}

	deviceID, err := strconv.ParseUint(deviceIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	var device models.Device
	result := db.GetDB().Preload("User").First(&device, deviceID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Device not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching device: %v", result.Error)
			http.Error(w, "Error fetching device", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

func createDevice(w http.ResponseWriter, r *http.Request) {
	var device models.Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate auth token if not provided
	if device.AuthToken == "" {
		token, err := auth.GenerateDeviceToken()
		if err != nil {
			log.Printf("Error generating device token: %v", err)
			http.Error(w, "Error generating device token", http.StatusInternalServerError)
			return
		}
		device.AuthToken = token
	}

	result := db.GetDB().Create(&device)
	if result.Error != nil {
		log.Printf("Error creating device: %v", result.Error)
		http.Error(w, "Error creating device", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(device)
}

func updateDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Device ID required", http.StatusBadRequest)
		return
	}

	deviceID, err := strconv.ParseUint(deviceIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	var device models.Device
	result := db.GetDB().First(&device, deviceID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Device not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching device", http.StatusInternalServerError)
		}
		return
	}

	var updateData models.Device
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update fields (don't update auth_token or id)
	device.Name = updateData.Name
	device.Description = updateData.Description
	device.DeviceType = updateData.DeviceType
	device.Location = updateData.Location
	device.IsActive = updateData.IsActive
	if updateData.UserID != nil {
		device.UserID = updateData.UserID
	}

	result = db.GetDB().Save(&device)
	if result.Error != nil {
		log.Printf("Error updating device: %v", result.Error)
		http.Error(w, "Error updating device", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

func deleteDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Device ID required", http.StatusBadRequest)
		return
	}

	deviceID, err := strconv.ParseUint(deviceIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Delete(&models.Device{}, deviceID)
	if result.Error != nil {
		log.Printf("Error deleting device: %v", result.Error)
		http.Error(w, "Error deleting device", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

