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

func ServicesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllServices(w, r)
	case "POST":
		if r.Header.Get("X-User-Role") != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		createService(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func ServiceHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getService(w, r)
	case "PUT":
		if r.Header.Get("X-User-Role") != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		updateService(w, r)
	case "DELETE":
		if r.Header.Get("X-User-Role") != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		deleteService(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getAllServices(w http.ResponseWriter, r *http.Request) {
	var services []models.Service
	query := db.GetDB()

	if active := r.URL.Query().Get("active"); active != "" {
		isActive := active == "true"
		query = query.Where("is_active = ?", isActive)
	}

	result := query.Find(&services)
	if result.Error != nil {
		log.Printf("Error fetching services: %v", result.Error)
		http.Error(w, "Error fetching services", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(services)
}

func createService(w http.ResponseWriter, r *http.Request) {
	var service models.Service
	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if service.Code == "" || service.Name == "" {
		http.Error(w, "Code and name are required", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Create(&service)
	if result.Error != nil {
		log.Printf("Error creating service: %v", result.Error)
		http.Error(w, "Error creating service", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(service)
}

func getService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Service ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var service models.Service
	result := db.GetDB().First(&service, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Service not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching service: %v", result.Error)
			http.Error(w, "Error fetching service", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(service)
}

func updateService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Service ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var service models.Service
	result := db.GetDB().First(&service, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Service not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching service", http.StatusInternalServerError)
		}
		return
	}

	var updateData models.Service
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	service.Code = updateData.Code
	service.Name = updateData.Name
	service.Description = updateData.Description
	service.IsActive = updateData.IsActive

	result = db.GetDB().Save(&service)
	if result.Error != nil {
		log.Printf("Error updating service: %v", result.Error)
		http.Error(w, "Error updating service", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(service)
}

func deleteService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Service ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Delete(&models.Service{}, id)
	if result.Error != nil {
		log.Printf("Error deleting service: %v", result.Error)
		http.Error(w, "Error deleting service", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
