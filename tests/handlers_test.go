package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Auto-migrate
	err = db.AutoMigrate(&User{}, &Device{}, &Signal{}, &SignalValue{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	// Set global DB for handlers
	DB = db

	return db
}

func TestLoginHandler_Success(t *testing.T) {
	// Setup test database
	testDB := setupTestDB(t)
	DB = testDB

	// Create test user
	user := User{
		Name:  "Test User",
		Email: "test@example.com",
	}
	err := user.SetPassword("password123")
	if err != nil {
		t.Fatalf("Failed to set password: %v", err)
	}
	testDB.Create(&user)

	// Create request
	loginData := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(loginData)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	LoginHandler(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response LoginResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Token == "" {
		t.Error("Token should not be empty")
	}

	if response.User.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", response.User.Email)
	}
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	// Setup test database
	testDB := setupTestDB(t)
	DB = testDB

	// Create test user
	user := User{
		Name:  "Test User",
		Email: "test@example.com",
	}
	err := user.SetPassword("password123")
	if err != nil {
		t.Fatalf("Failed to set password: %v", err)
	}
	testDB.Create(&user)

	// Create request with wrong password
	loginData := map[string]string{
		"email":    "test@example.com",
		"password": "wrongpassword",
	}
	jsonData, _ := json.Marshal(loginData)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	LoginHandler(w, req)

	// Assert
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRegisterDeviceHandler(t *testing.T) {
	// Setup test database
	testDB := setupTestDB(t)
	DB = testDB

	// Create test user
	user := User{
		Name:  "Test User",
		Email: "test@example.com",
	}
	err := user.SetPassword("password123")
	if err != nil {
		t.Fatalf("Failed to set password: %v", err)
	}
	testDB.Create(&user)

	// Generate JWT token
	token, err := GenerateJWT(user.ID, user.Email)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create request
	deviceData := map[string]string{
		"name":        "Test Device",
		"description": "Test Description",
		"device_type": "sensor",
		"location":    "Test Location",
	}
	jsonData, _ := json.Marshal(deviceData)
	req := httptest.NewRequest("POST", "/auth/register-device", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-User-ID", "1")
	w := httptest.NewRecorder()

	// Execute
	RegisterDeviceHandler(w, req)

	// Assert
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response RegisterDeviceResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Device.Name != "Test Device" {
		t.Errorf("Expected device name 'Test Device', got %s", response.Device.Name)
	}

	if response.AuthToken == "" {
		t.Error("Auth token should not be empty")
	}
}

func TestGetAllDevices(t *testing.T) {
	// Setup test database
	testDB := setupTestDB(t)
	DB = testDB

	// Create test devices
	device1 := Device{Name: "Device 1", AuthToken: "token1", IsActive: true}
	device2 := Device{Name: "Device 2", AuthToken: "token2", IsActive: true}
	testDB.Create(&device1)
	testDB.Create(&device2)

	// Create request
	req := httptest.NewRequest("GET", "/devices", nil)
	w := httptest.NewRecorder()

	// Execute
	getAllDevices(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var devices []Device
	err := json.Unmarshal(w.Body.Bytes(), &devices)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(devices))
	}
}

