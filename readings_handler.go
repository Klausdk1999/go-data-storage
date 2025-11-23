package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

// ReadingsHandler is a legacy handler for backward compatibility
// It redirects to signal values
func ReadingsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllReadings(w, r)
	case "POST":
		createReading(w, r)
	default:
		http.Error(w, "Unsupported request method.", http.StatusMethodNotAllowed)
	}
}

func getAllReadings(w http.ResponseWriter, r *http.Request) {
	// Redirect to signal values
	var signalValues []SignalValue
	query := DB.Preload("Signal").Preload("Signal.Device").Preload("User")

	// Limit results
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
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}

	readingsBytes, err := json.MarshalIndent(signalValues, "", "\t")
	if err != nil {
		http.Error(w, "Error marshaling readings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(readingsBytes)
}

func createReading(w http.ResponseWriter, r *http.Request) {
	// Legacy endpoint - redirect to signal values
	// This maintains backward compatibility
	var readingData struct {
		UserID       uint     `json:"user_id"`
		Value        *float64 `json:"value"`
		DigitalValue *bool    `json:"digital_value"`
		SignalID     uint     `json:"signal_id"`
	}

	err := json.NewDecoder(r.Body).Decode(&readingData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// If signal_id is not provided, this is legacy format
	// We can't create without signal_id in new structure
	if readingData.SignalID == 0 {
		http.Error(w, "signal_id is required. Please use /signal-values endpoint", http.StatusBadRequest)
		return
	}

	// Create as signal value
	signalValue := SignalValue{
		SignalID:     readingData.SignalID,
		UserID:       func() *uint { if readingData.UserID > 0 { u := readingData.UserID; return &u }; return nil }(),
		Value:        readingData.Value,
		DigitalValue: readingData.DigitalValue,
		Timestamp:    time.Now(),
	}

	result := DB.Create(&signalValue)
	if result.Error != nil {
		log.Printf("Error creating reading: %v", result.Error)
		http.Error(w, result.Error.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(signalValue)
}

