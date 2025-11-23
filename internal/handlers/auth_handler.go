package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"data-storage/internal/auth"
	"data-storage/internal/db"
	"data-storage/internal/models"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  models.User  `json:"user"`
}

type RegisterDeviceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DeviceType  string `json:"device_type,omitempty"`
	Location    string `json:"location,omitempty"`
}

type RegisterDeviceResponse struct {
	Device    models.Device `json:"device"`
	AuthToken string        `json:"auth_token"`
}

// LoginHandler handles user authentication
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Find user by email
	var user models.User
	result := db.GetDB().Where("email = ? AND is_active = ?", req.Email, true).First(&user)
	if result.Error != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Check password
	if !user.CheckPassword(req.Password) {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		log.Printf("Error generating JWT: %v", err)
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	// Clear password hash from response
	user.PasswordHash = ""

	response := LoginResponse{
		Token: token,
		User:  user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RegisterDeviceHandler allows authenticated users to register new devices
func RegisterDeviceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from auth middleware (set by RequireUserAuth)
	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req RegisterDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate device auth token
	authToken, err := auth.GenerateDeviceToken()
	if err != nil {
		log.Printf("Error generating device token: %v", err)
		http.Error(w, "Error generating device token", http.StatusInternalServerError)
		return
	}

	// Create device
	device := models.Device{
		Name:        req.Name,
		Description: req.Description,
		DeviceType:  req.DeviceType,
		Location:    req.Location,
		UserID:      func() *uint { u := uint(userID); return &u }(),
		AuthToken:   authToken,
		IsActive:    true,
	}

	result := db.GetDB().Create(&device)
	if result.Error != nil {
		log.Printf("Error creating device: %v", result.Error)
		http.Error(w, "Error creating device", http.StatusInternalServerError)
		return
	}

	response := RegisterDeviceResponse{
		Device:    device,
		AuthToken: authToken,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

