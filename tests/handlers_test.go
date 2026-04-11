package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"data-storage/internal/auth"
	"data-storage/internal/db"
	"data-storage/internal/handlers"
	"data-storage/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	err = testDB.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.Signal{},
		&models.SignalValue{},
		&models.Product{},
		&models.RawMaterial{},
		&models.BillOfMaterials{},
		&models.Customer{},
		&models.ProductionOrder{},
		&models.StockMovement{},
		&models.Service{},
		&models.TimeEntry{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	db.DB = testDB
	return testDB
}

func createTestUser(t *testing.T, testDB *gorm.DB, name, email, password, userType string) models.User {
	t.Helper()
	user := models.User{
		Name:     name,
		Email:    email,
		Type:     userType,
		IsActive: true,
	}
	if err := user.SetPassword(password); err != nil {
		t.Fatalf("Failed to set password: %v", err)
	}
	testDB.Create(&user)
	return user
}

func TestLoginHandler_Success(t *testing.T) {
	testDB := setupTestDB(t)
	createTestUser(t, testDB, "Admin User", "admin@test.com", "password123", "admin")

	loginData := map[string]string{
		"email":    "admin@test.com",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(loginData)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.LoginHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response struct {
		Token string      `json:"token"`
		User  models.User `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Token == "" {
		t.Error("Token should not be empty")
	}
	if response.User.Email != "admin@test.com" {
		t.Errorf("Expected email admin@test.com, got %s", response.User.Email)
	}
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	testDB := setupTestDB(t)
	createTestUser(t, testDB, "Admin User", "admin@test.com", "password123", "admin")

	loginData := map[string]string{
		"email":    "admin@test.com",
		"password": "wrongpassword",
	}
	jsonData, _ := json.Marshal(loginData)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.LoginHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestLoginHandler_NonexistentUser(t *testing.T) {
	setupTestDB(t)

	loginData := map[string]string{
		"email":    "nobody@test.com",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(loginData)
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.LoginHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRegisterDeviceHandler(t *testing.T) {
	testDB := setupTestDB(t)
	user := createTestUser(t, testDB, "Admin User", "admin@test.com", "password123", "admin")

	token, err := auth.GenerateJWT(user.ID, user.Email, user.Type)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

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

	handlers.RegisterDeviceHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response struct {
		Device    models.Device `json:"device"`
		AuthToken string        `json:"auth_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
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
	testDB := setupTestDB(t)

	testDB.Create(&models.Device{Name: "Device 1", AuthToken: "token1", IsActive: true})
	testDB.Create(&models.Device{Name: "Device 2", AuthToken: "token2", IsActive: true})

	req := httptest.NewRequest("GET", "/devices", nil)
	w := httptest.NewRecorder()

	handlers.DevicesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var devices []models.Device
	if err := json.Unmarshal(w.Body.Bytes(), &devices); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(devices))
	}
}

func TestWorkerCannotAccessUsersEndpoint(t *testing.T) {
	testDB := setupTestDB(t)
	worker := createTestUser(t, testDB, "Worker", "worker@test.com", "pass123", "worker")

	token, err := auth.GenerateJWT(worker.ID, worker.Email, worker.Type)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	// Simulate RequireAdmin middleware
	handler := auth.RequireAdmin(handlers.UsersHandler)
	handler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for worker accessing users, got %d", w.Code)
	}
}

func TestAdminCanAccessUsersEndpoint(t *testing.T) {
	testDB := setupTestDB(t)
	admin := createTestUser(t, testDB, "Admin", "admin@test.com", "pass123", "admin")

	token, err := auth.GenerateJWT(admin.ID, admin.Email, admin.Type)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := auth.RequireAdmin(handlers.UsersHandler)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for admin accessing users, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// --- Generic Device Data Handler Tests ---

func createTestDevice(t *testing.T, testDB *gorm.DB, name, token string) models.Device {
	t.Helper()
	device := models.Device{
		Name:      name,
		AuthToken: token,
		IsActive:  true,
	}
	testDB.Create(&device)
	return device
}

func postDeviceData(t *testing.T, payload map[string]interface{}, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/devices/data", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	handlers.GenericDataHandler(w, req)
	return w
}

func getSignalsForDevice(t *testing.T, testDB *gorm.DB, deviceID uint) []models.Signal {
	t.Helper()
	var signals []models.Signal
	testDB.Where("device_id = ?", deviceID).Find(&signals)
	return signals
}

func TestGenericDataHandler_CustomFieldNames(t *testing.T) {
	testDB := setupTestDB(t)
	device := createTestDevice(t, testDB, "esp32-test", "test-token-123")
	os.Setenv("DEVICE_AUTO_CREATE", "true")

	w := postDeviceData(t, map[string]interface{}{
		"device_id":    "esp32-test",
		"temperatura":  23.5,
		"umidade":      61.0,
		"motor_ligado": true,
	}, "Bearer test-token-123")

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	signals := getSignalsForDevice(t, testDB, device.ID)
	if len(signals) != 3 {
		t.Fatalf("Expected 3 signals, got %d", len(signals))
	}

	names := map[string]string{}
	for _, s := range signals {
		names[s.Name] = s.SignalType
	}

	if names["temperatura"] != "analogic" {
		t.Errorf("Expected 'temperatura' as analogic signal, got %q", names["temperatura"])
	}
	if names["umidade"] != "analogic" {
		t.Errorf("Expected 'umidade' as analogic signal, got %q", names["umidade"])
	}
	if names["motor_ligado"] != "digital" {
		t.Errorf("Expected 'motor_ligado' as digital signal, got %q", names["motor_ligado"])
	}
}

func TestGenericDataHandler_LegacyFieldNames(t *testing.T) {
	testDB := setupTestDB(t)
	device := createTestDevice(t, testDB, "esp32-legacy", "legacy-token")

	w := postDeviceData(t, map[string]interface{}{
		"device_id": "esp32-legacy",
		"field_1":   10.0,
		"field_2":   20.0,
	}, "Bearer legacy-token")

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	signals := getSignalsForDevice(t, testDB, device.ID)
	if len(signals) != 2 {
		t.Fatalf("Expected 2 signals, got %d", len(signals))
	}

	names := map[string]bool{}
	for _, s := range signals {
		names[s.Name] = true
	}
	if !names["field_1"] || !names["field_2"] {
		t.Errorf("Expected field_1 and field_2 signals, got %v", names)
	}
}

func TestGenericDataHandler_MixedCustomAndLegacy(t *testing.T) {
	testDB := setupTestDB(t)
	device := createTestDevice(t, testDB, "esp32-mixed", "mixed-token")

	w := postDeviceData(t, map[string]interface{}{
		"device_id":   "esp32-mixed",
		"field_1":     10.0,
		"temperatura": 25.0,
	}, "Bearer mixed-token")

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	signals := getSignalsForDevice(t, testDB, device.ID)
	if len(signals) != 2 {
		t.Fatalf("Expected 2 signals, got %d", len(signals))
	}

	names := map[string]bool{}
	for _, s := range signals {
		names[s.Name] = true
	}
	if !names["field_1"] || !names["temperatura"] {
		t.Errorf("Expected field_1 and temperatura, got %v", names)
	}
}

func TestGenericDataHandler_NormalizedMatching(t *testing.T) {
	testDB := setupTestDB(t)
	device := createTestDevice(t, testDB, "esp32-norm", "norm-token")

	// First POST creates signal with lowercase name
	w := postDeviceData(t, map[string]interface{}{
		"device_id":   "esp32-norm",
		"temperatura": 20.0,
	}, "Bearer norm-token")
	if w.Code != http.StatusCreated {
		t.Fatalf("First POST: expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	signalsBefore := getSignalsForDevice(t, testDB, device.ID)
	if len(signalsBefore) != 1 {
		t.Fatalf("Expected 1 signal after first POST, got %d", len(signalsBefore))
	}

	// Second POST with different casing — should match existing signal, not create new
	w = postDeviceData(t, map[string]interface{}{
		"device_id":   "esp32-norm",
		"Temperatura": 25.0,
	}, "Bearer norm-token")
	if w.Code != http.StatusCreated {
		t.Fatalf("Second POST: expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	signalsAfter := getSignalsForDevice(t, testDB, device.ID)
	if len(signalsAfter) != 1 {
		t.Errorf("Expected 1 signal after normalized match, got %d (duplicate created)", len(signalsAfter))
	}

	// Verify two values stored for the same signal
	var count int64
	testDB.Model(&models.SignalValue{}).Where("signal_id = ?", signalsBefore[0].ID).Count(&count)
	if count != 2 {
		t.Errorf("Expected 2 values for signal, got %d", count)
	}
}

func TestGenericDataHandler_NormalizedMatchingWithSpaces(t *testing.T) {
	testDB := setupTestDB(t)
	device := createTestDevice(t, testDB, "esp32-spaces", "spaces-token")

	// Create signal with spaces in name
	testDB.Create(&models.Signal{
		DeviceID:   device.ID,
		Name:       "Nivel Rio",
		SignalType: "analogic",
		Direction:  "input",
		IsActive:   true,
	})

	// POST with no spaces — should match "Nivel Rio" via normalization
	w := postDeviceData(t, map[string]interface{}{
		"device_id": "esp32-spaces",
		"nivelrio":  3.5,
	}, "Bearer spaces-token")
	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	signals := getSignalsForDevice(t, testDB, device.ID)
	if len(signals) != 1 {
		t.Errorf("Expected 1 signal (matched via normalization), got %d", len(signals))
	}
}

func TestGenericDataHandler_Unauthorized(t *testing.T) {
	setupTestDB(t)

	w := postDeviceData(t, map[string]interface{}{
		"device_id":   "esp32-test",
		"temperatura": 20.0,
	}, "Bearer invalid-token")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestGenericDataHandler_NoAuth(t *testing.T) {
	setupTestDB(t)

	w := postDeviceData(t, map[string]interface{}{
		"device_id":   "esp32-test",
		"temperatura": 20.0,
	}, "")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestGenericDataHandler_MissingDeviceID(t *testing.T) {
	testDB := setupTestDB(t)
	createTestDevice(t, testDB, "esp32-test", "token-123")

	w := postDeviceData(t, map[string]interface{}{
		"temperatura": 20.0,
	}, "Bearer token-123")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestGenericDataHandler_ApiKeyAuth(t *testing.T) {
	testDB := setupTestDB(t)
	os.Setenv("DEVICE_API_KEY", "test-api-key-123")
	os.Setenv("DEVICE_AUTO_CREATE", "true")
	defer os.Unsetenv("DEVICE_API_KEY")

	payload, _ := json.Marshal(map[string]interface{}{
		"device_id":   "esp32-apikey",
		"temperatura": 22.0,
	})
	req := httptest.NewRequest("POST", "/devices/data", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-api-key-123")
	w := httptest.NewRecorder()
	handlers.GenericDataHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify device was auto-created
	var device models.Device
	testDB.Where("name = ?", "esp32-apikey").First(&device)
	if device.ID == 0 {
		t.Error("Device should have been auto-created")
	}

	signals := getSignalsForDevice(t, testDB, device.ID)
	if len(signals) != 1 || signals[0].Name != "temperatura" {
		t.Errorf("Expected 1 signal named 'temperatura', got %v", signals)
	}
}
