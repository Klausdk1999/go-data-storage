package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func UserReadingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract user ID from URL path using mux
	vars := mux.Vars(r)
	userIDStr, ok := vars["user_id"]
	if !ok {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Query database for signal values belonging to the user
	var signalValues []SignalValue
	result := DB.Where("user_id = ?", uint(userID)).Preload("Signal").Preload("Signal.Device").Order("timestamp DESC").Find(&signalValues)
	if result.Error != nil {
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}

	signalsBytes, err := json.MarshalIndent(signalValues, "", "\t")
	if err != nil {
		http.Error(w, "Error marshaling signal values", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(signalsBytes)
}

