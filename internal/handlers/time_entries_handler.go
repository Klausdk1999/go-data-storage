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

func TimeEntriesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllTimeEntries(w, r)
	case "POST":
		createTimeEntry(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func TimeEntryHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getTimeEntry(w, r)
	case "PUT":
		updateTimeEntry(w, r)
	case "DELETE":
		deleteTimeEntry(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getAllTimeEntries(w http.ResponseWriter, r *http.Request) {
	var entries []models.TimeEntry
	query := db.GetDB().Preload("User").Preload("ProductionOrder.Product").Preload("Service")

	if userID := r.URL.Query().Get("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if orderID := r.URL.Query().Get("production_order_id"); orderID != "" {
		query = query.Where("production_order_id = ?", orderID)
	}

	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		query = query.Where("day >= ?", fromDate)
	}

	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		query = query.Where("day <= ?", toDate)
	}

	// Workers can only see their own entries
	role := r.Header.Get("X-User-Role")
	if role != "admin" {
		userIDStr := r.Header.Get("X-User-ID")
		query = query.Where("user_id = ?", userIDStr)
	}

	result := query.Order("day DESC, start_time DESC").Find(&entries)
	if result.Error != nil {
		log.Printf("Error fetching time entries: %v", result.Error)
		http.Error(w, "Error fetching time entries", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// batchCreateResponse is returned when creating multiple entries at once.
type batchCreateResponse struct {
	Created []models.TimeEntry `json:"created"`
	Errors  []batchEntryError  `json:"errors,omitempty"`
}

type batchEntryError struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
}

func createTimeEntry(w http.ResponseWriter, r *http.Request) {
	role := r.Header.Get("X-User-Role")
	userIDStr := r.Header.Get("X-User-ID")

	// Read raw body to detect whether it is an object or an array.
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Determine payload shape by inspecting the first non-whitespace byte.
	isArray := false
	for _, b := range raw {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		isArray = b == '['
		break
	}

	if isArray {
		// ── Batch mode ────────────────────────────────────────────────────
		var inputs []models.TimeEntry
		if err := json.Unmarshal(raw, &inputs); err != nil {
			http.Error(w, "Invalid request body: expected array of time entries", http.StatusBadRequest)
			return
		}

		if len(inputs) == 0 {
			http.Error(w, "At least one time entry is required", http.StatusBadRequest)
			return
		}

		resp := batchCreateResponse{
			Created: make([]models.TimeEntry, 0, len(inputs)),
			Errors:  []batchEntryError{},
		}

		for i, input := range inputs {
			created, errMsg := processSingleEntry(input, role, userIDStr)
			if errMsg != "" {
				resp.Errors = append(resp.Errors, batchEntryError{Index: i, Message: errMsg})
				continue
			}
			resp.Created = append(resp.Created, *created)
		}

		if len(resp.Created) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// ── Single entry mode (retrocompatível) ───────────────────────────────
	var input models.TimeEntry
	if err := json.Unmarshal(raw, &input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	created, errMsg := processSingleEntry(input, role, userIDStr)
	if errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

// processSingleEntry validates and persists a single TimeEntry.
// Returns the created entry (with associations preloaded) or an error message.
func processSingleEntry(entry models.TimeEntry, role, userIDStr string) (*models.TimeEntry, string) {
	// Workers can only create entries for themselves.
	if role != "admin" {
		uid, _ := strconv.ParseUint(userIDStr, 10, 32)
		entry.UserID = uint(uid)
	}

	if entry.UserID == 0 || entry.ProductionOrderID == 0 || entry.ServiceID == 0 {
		return nil, "user_id, production_order_id, and service_id are required"
	}

	if entry.Day == "" || entry.StartTime == "" || entry.EndTime == "" {
		return nil, "day, start_time, and end_time are required"
	}

	result := db.GetDB().Create(&entry)
	if result.Error != nil {
		log.Printf("Error creating time entry: %v", result.Error)
		return nil, "Error creating time entry"
	}

	db.GetDB().Preload("User").Preload("ProductionOrder.Product").Preload("Service").First(&entry, entry.ID)
	return &entry, ""
}

func getTimeEntry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Time entry ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid time entry ID", http.StatusBadRequest)
		return
	}

	var entry models.TimeEntry
	result := db.GetDB().Preload("User").Preload("ProductionOrder.Product").Preload("Service").First(&entry, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Time entry not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching time entry: %v", result.Error)
			http.Error(w, "Error fetching time entry", http.StatusInternalServerError)
		}
		return
	}

	// Workers can only view their own entries
	role := r.Header.Get("X-User-Role")
	if role != "admin" {
		userIDStr := r.Header.Get("X-User-ID")
		uid, _ := strconv.ParseUint(userIDStr, 10, 32)
		if entry.UserID != uint(uid) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func updateTimeEntry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Time entry ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid time entry ID", http.StatusBadRequest)
		return
	}

	var entry models.TimeEntry
	result := db.GetDB().First(&entry, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Time entry not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching time entry", http.StatusInternalServerError)
		}
		return
	}

	// Workers can only modify their own entries
	role := r.Header.Get("X-User-Role")
	if role != "admin" {
		userIDStr := r.Header.Get("X-User-ID")
		uid, _ := strconv.ParseUint(userIDStr, 10, 32)
		if entry.UserID != uint(uid) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	var updateData models.TimeEntry
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	entry.ProductionOrderID = updateData.ProductionOrderID
	entry.ServiceID = updateData.ServiceID
	entry.Day = updateData.Day
	entry.StartTime = updateData.StartTime
	entry.EndTime = updateData.EndTime
	entry.Observations = updateData.Observations

	result = db.GetDB().Save(&entry)
	if result.Error != nil {
		log.Printf("Error updating time entry: %v", result.Error)
		http.Error(w, "Error updating time entry", http.StatusInternalServerError)
		return
	}

	db.GetDB().Preload("User").Preload("ProductionOrder.Product").Preload("Service").First(&entry, entry.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func deleteTimeEntry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Time entry ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid time entry ID", http.StatusBadRequest)
		return
	}

	// Workers can only delete their own entries
	role := r.Header.Get("X-User-Role")
	if role != "admin" {
		var entry models.TimeEntry
		if err := db.GetDB().First(&entry, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				http.Error(w, "Time entry not found", http.StatusNotFound)
			} else {
				http.Error(w, "Error fetching time entry", http.StatusInternalServerError)
			}
			return
		}
		userIDStr := r.Header.Get("X-User-ID")
		uid, _ := strconv.ParseUint(userIDStr, 10, 32)
		if entry.UserID != uint(uid) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	result := db.GetDB().Delete(&models.TimeEntry{}, id)
	if result.Error != nil {
		log.Printf("Error deleting time entry: %v", result.Error)
		http.Error(w, "Error deleting time entry", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "Time entry not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
