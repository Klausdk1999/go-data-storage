package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"data-storage/internal/auth"
	"data-storage/internal/db"
	"data-storage/internal/handlers"
	"data-storage/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
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

// ── Time Entry helpers ────────────────────────────────────────────────────────

func setupTimeEntryDeps(t *testing.T, testDB *gorm.DB) (user models.User, order models.ProductionOrder, svc models.Service) {
	t.Helper()

	user = createTestUser(t, testDB, "Worker", "worker@te.com", "pass", "worker")

	product := models.Product{Name: "Prod", SKU: "SKU-1", IsActive: true}
	testDB.Create(&product)

	order = models.ProductionOrder{ProductID: product.ID, Quantity: 10, Status: "planned"}
	testDB.Create(&order)

	svc = models.Service{Code: "SVC-1", Name: "Montagem", IsActive: true}
	testDB.Create(&svc)

	return
}

func makeTimeEntryRequest(t *testing.T, body interface{}, userID uint, role string) *httptest.ResponseRecorder {
	t.Helper()
	jsonData, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	req := httptest.NewRequest("POST", "/time-entries", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", role)
	req.Header.Set("X-User-ID", strconv.Itoa(int(userID)))
	w := httptest.NewRecorder()
	handlers.TimeEntriesHandler(w, req)
	return w
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestCreateTimeEntry_Single(t *testing.T) {
	testDB := setupTestDB(t)
	user, order, svc := setupTimeEntryDeps(t, testDB)

	payload := map[string]interface{}{
		"production_order_id": order.ID,
		"service_id":          svc.ID,
		"day":                 "2024-01-15",
		"start_time":          "08:00",
		"end_time":            "12:00",
	}
	w := makeTimeEntryRequest(t, payload, user.ID, "worker")

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var entry models.TimeEntry
	if err := json.Unmarshal(w.Body.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if entry.ID == 0 {
		t.Error("Expected created entry to have an ID")
	}
	if entry.UserID != user.ID {
		t.Errorf("Expected user_id %d, got %d", user.ID, entry.UserID)
	}
}

func TestCreateTimeEntry_Batch(t *testing.T) {
	testDB := setupTestDB(t)
	user, order, svc := setupTimeEntryDeps(t, testDB)

	payload := []map[string]interface{}{
		{
			"production_order_id": order.ID,
			"service_id":          svc.ID,
			"day":                 "2024-01-15",
			"start_time":          "08:00",
			"end_time":            "12:00",
		},
		{
			"production_order_id": order.ID,
			"service_id":          svc.ID,
			"day":                 "2024-01-16",
			"start_time":          "08:00",
			"end_time":            "17:00",
		},
	}
	w := makeTimeEntryRequest(t, payload, user.ID, "worker")

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Created []models.TimeEntry `json:"created"`
		Errors  []interface{}      `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(resp.Created) != 2 {
		t.Errorf("Expected 2 created entries, got %d", len(resp.Created))
	}
	if len(resp.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(resp.Errors))
	}
}

func TestCreateTimeEntry_BatchPartialError(t *testing.T) {
	testDB := setupTestDB(t)
	user, order, svc := setupTimeEntryDeps(t, testDB)

	// Second entry is missing required "day" field.
	payload := []map[string]interface{}{
		{
			"production_order_id": order.ID,
			"service_id":          svc.ID,
			"day":                 "2024-01-15",
			"start_time":          "08:00",
			"end_time":            "12:00",
		},
		{
			"production_order_id": order.ID,
			"service_id":          svc.ID,
			// "day" intentionally omitted
			"start_time": "13:00",
			"end_time":   "17:00",
		},
	}
	w := makeTimeEntryRequest(t, payload, user.ID, "worker")

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201 (partial success), got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Created []models.TimeEntry `json:"created"`
		Errors  []interface{}      `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(resp.Created) != 1 {
		t.Errorf("Expected 1 created entry, got %d", len(resp.Created))
	}
	if len(resp.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(resp.Errors))
	}
}

func TestCreateTimeEntry_EmptyBatch(t *testing.T) {
	testDB := setupTestDB(t)
	user, _, _ := setupTimeEntryDeps(t, testDB)

	payload := []map[string]interface{}{}
	w := makeTimeEntryRequest(t, payload, user.ID, "worker")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty array, got %d", w.Code)
	}
}

func TestCreateTimeEntry_MissingFields(t *testing.T) {
	testDB := setupTestDB(t)
	user, order, _ := setupTimeEntryDeps(t, testDB)

	// Missing service_id, day, start_time, end_time.
	payload := map[string]interface{}{
		"production_order_id": order.ID,
	}
	w := makeTimeEntryRequest(t, payload, user.ID, "worker")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing fields, got %d", w.Code)
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

// --- User Preferences Tests ---

func TestGetUserPreferences(t *testing.T) {
	testDB := setupTestDB(t)
	user := createTestUser(t, testDB, "Prefs User", "prefs@test.com", "pass123", "worker")

	// Set preferences directly in DB
	prefs := models.JSONB{"theme": "dark", "language": "pt-BR"}
	testDB.Model(&user).Update("preferences", prefs)

	req := httptest.NewRequest("GET", "/users/"+strconv.Itoa(int(user.ID))+"/preferences", nil)
	req.Header.Set("X-User-ID", strconv.Itoa(int(user.ID)))
	req.Header.Set("X-User-Role", "worker")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(user.ID))})
	w := httptest.NewRecorder()

	handlers.UserPreferencesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result["theme"] != "dark" {
		t.Errorf("Expected theme 'dark', got %v", result["theme"])
	}
	if result["language"] != "pt-BR" {
		t.Errorf("Expected language 'pt-BR', got %v", result["language"])
	}
}

func TestPutUserPreferences(t *testing.T) {
	testDB := setupTestDB(t)
	user := createTestUser(t, testDB, "Prefs User", "prefs@test.com", "pass123", "worker")

	// Set initial preferences
	initial := models.JSONB{"theme": "light", "language": "en"}
	testDB.Model(&user).Update("preferences", initial)

	// PUT new preferences (full overwrite — frontend sends complete state)
	newPrefs := map[string]interface{}{"theme": "dark", "language": "en", "sidebar_collapsed": true}
	jsonData, _ := json.Marshal(newPrefs)
	req := httptest.NewRequest("PUT", "/users/"+strconv.Itoa(int(user.ID))+"/preferences", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", strconv.Itoa(int(user.ID)))
	req.Header.Set("X-User-Role", "worker")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(user.ID))})
	w := httptest.NewRecorder()

	handlers.UserPreferencesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify in DB — full overwrite stores exactly what was sent
	var updated models.User
	testDB.First(&updated, user.ID)

	if updated.Preferences["theme"] != "dark" {
		t.Errorf("Expected theme 'dark', got %v", updated.Preferences["theme"])
	}
	if updated.Preferences["language"] != "en" {
		t.Errorf("Expected language 'en', got %v", updated.Preferences["language"])
	}
	if updated.Preferences["sidebar_collapsed"] != true {
		t.Errorf("Expected sidebar_collapsed true, got %v", updated.Preferences["sidebar_collapsed"])
	}
}

func TestUserPreferences_CannotAccessOtherUser(t *testing.T) {
	testDB := setupTestDB(t)
	user1 := createTestUser(t, testDB, "User One", "user1@test.com", "pass123", "worker")
	user2 := createTestUser(t, testDB, "User Two", "user2@test.com", "pass123", "worker")

	// User 2 tries to access User 1's preferences
	req := httptest.NewRequest("GET", "/users/"+strconv.Itoa(int(user1.ID))+"/preferences", nil)
	req.Header.Set("X-User-ID", strconv.Itoa(int(user2.ID)))
	req.Header.Set("X-User-Role", "worker")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(user1.ID))})
	w := httptest.NewRecorder()

	handlers.UserPreferencesHandler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 when worker accesses another user's preferences, got %d", w.Code)
	}
}

// --- Image Handler Tests ---

// createMultipartImageRequest builds a multipart form request with an "image" file field.
func createMultipartImageRequest(t *testing.T, method, url string, imageData []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("image", "test.png")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	_, err = part.Write(imageData)
	if err != nil {
		t.Fatalf("Failed to write image data: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// smallPNG returns a minimal valid PNG file (1x1 pixel, transparent).
func smallPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x62, 0x00, 0x00, 0x00, 0x02,
		0x00, 0x01, 0xe5, 0x27, 0xde, 0xfc, 0x00, 0x00,
		0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42,
		0x60, 0x82, // IEND chunk
	}
}

func TestImageUpload_Product(t *testing.T) {
	testDB := setupTestDB(t)

	product := models.Product{Name: "Test Product", SKU: "IMG-1", Category: "test", IsActive: true}
	testDB.Create(&product)

	imageData := smallPNG()
	req := createMultipartImageRequest(t, "PUT", "/products/"+strconv.Itoa(int(product.ID))+"/image", imageData)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(product.ID))})
	w := httptest.NewRecorder()

	handler := handlers.ImageUploadHandler("products")
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", resp["status"])
	}

	// Verify image was stored in DB
	var saved models.Product
	testDB.First(&saved, product.ID)
	if len(saved.Image) == 0 {
		t.Error("Expected image to be stored in DB")
	}
	if saved.ImageType != "image/png" {
		t.Errorf("Expected image_type 'image/png', got %q", saved.ImageType)
	}
}

func TestImageUpload_TooLarge(t *testing.T) {
	testDB := setupTestDB(t)

	product := models.Product{Name: "Big Image Product", SKU: "IMG-BIG", Category: "test", IsActive: true}
	testDB.Create(&product)

	// Create 3MB of data (exceeds 2MB limit)
	largeData := make([]byte, 3*1024*1024)
	for i := range largeData {
		largeData[i] = 0xFF
	}

	req := createMultipartImageRequest(t, "PUT", "/products/"+strconv.Itoa(int(product.ID))+"/image", largeData)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(product.ID))})
	w := httptest.NewRecorder()

	handler := handlers.ImageUploadHandler("products")
	handler(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected 413 for too-large image, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestImageDownload_Product(t *testing.T) {
	testDB := setupTestDB(t)

	imageData := smallPNG()
	product := models.Product{
		Name:      "Download Test",
		SKU:       "IMG-DL",
		Category:  "test",
		IsActive:  true,
		Image:     imageData,
		ImageType: "image/png",
	}
	testDB.Create(&product)

	req := httptest.NewRequest("GET", "/products/"+strconv.Itoa(int(product.ID))+"/image", nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(product.ID))})
	w := httptest.NewRecorder()

	handler := handlers.ImageDownloadHandler("products")
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Expected Content-Type 'image/png', got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("Expected Cache-Control 'public, max-age=3600', got %q", cc)
	}

	body, _ := io.ReadAll(w.Body)
	if !bytes.Equal(body, imageData) {
		t.Errorf("Downloaded image bytes do not match original (got %d bytes, want %d)", len(body), len(imageData))
	}
}

func TestImageDelete_Product(t *testing.T) {
	testDB := setupTestDB(t)

	product := models.Product{
		Name:      "Delete Test",
		SKU:       "IMG-DEL",
		Category:  "test",
		IsActive:  true,
		Image:     smallPNG(),
		ImageType: "image/png",
	}
	testDB.Create(&product)

	req := httptest.NewRequest("DELETE", "/products/"+strconv.Itoa(int(product.ID))+"/image", nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(product.ID))})
	w := httptest.NewRecorder()

	handler := handlers.ImageDeleteHandler("products")
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", resp["status"])
	}

	// Verify image was removed from DB
	var saved models.Product
	testDB.First(&saved, product.ID)
	if len(saved.Image) != 0 {
		t.Errorf("Expected image to be nil after delete, got %d bytes", len(saved.Image))
	}
	if saved.ImageType != "" {
		t.Errorf("Expected image_type to be empty after delete, got %q", saved.ImageType)
	}
}

// ── Products CRUD Tests ──────────────────────────────────────────────────────

func TestGetAllProducts(t *testing.T) {
	testDB := setupTestDB(t)

	testDB.Create(&models.Product{Name: "Product A", SKU: "SKU-A", Category: "cat1", IsActive: true})
	testDB.Create(&models.Product{Name: "Product B", SKU: "SKU-B", Category: "cat2", IsActive: true})

	req := httptest.NewRequest("GET", "/products", nil)
	w := httptest.NewRecorder()

	handlers.ProductsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var products []models.Product
	if err := json.Unmarshal(w.Body.Bytes(), &products); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(products) != 2 {
		t.Errorf("Expected 2 products, got %d", len(products))
	}
}

func TestCreateProduct(t *testing.T) {
	setupTestDB(t)

	payload := map[string]interface{}{
		"name":      "New Product",
		"sku":       "SKU-NEW",
		"category":  "electronics",
		"is_active": true,
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/products", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.ProductsHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var product models.Product
	if err := json.Unmarshal(w.Body.Bytes(), &product); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if product.Name != "New Product" {
		t.Errorf("Expected name 'New Product', got %q", product.Name)
	}
	if product.SKU != "SKU-NEW" {
		t.Errorf("Expected SKU 'SKU-NEW', got %q", product.SKU)
	}
	if product.Category != "electronics" {
		t.Errorf("Expected category 'electronics', got %q", product.Category)
	}
	if !product.IsActive {
		t.Error("Expected is_active to be true")
	}
}

func TestCreateProduct_MissingName(t *testing.T) {
	setupTestDB(t)

	payload := map[string]interface{}{
		"sku":      "SKU-NO-NAME",
		"category": "test",
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/products", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.ProductsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestGetProductByID(t *testing.T) {
	testDB := setupTestDB(t)

	product := models.Product{Name: "Lookup Product", SKU: "SKU-LOOK", Category: "test", IsActive: true}
	testDB.Create(&product)

	req := httptest.NewRequest("GET", "/products/"+strconv.Itoa(int(product.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(product.ID))})
	w := httptest.NewRecorder()

	handlers.ProductHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.Product
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.Name != "Lookup Product" {
		t.Errorf("Expected name 'Lookup Product', got %q", result.Name)
	}
	if result.SKU != "SKU-LOOK" {
		t.Errorf("Expected SKU 'SKU-LOOK', got %q", result.SKU)
	}
}

func TestGetProductByID_NotFound(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest("GET", "/products/9999", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "9999"})
	w := httptest.NewRecorder()

	handlers.ProductHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProduct(t *testing.T) {
	testDB := setupTestDB(t)

	product := models.Product{Name: "Old Name", SKU: "SKU-OLD", Category: "old", IsActive: true}
	testDB.Create(&product)

	payload := map[string]interface{}{
		"name":      "Updated Name",
		"sku":       "SKU-UPD",
		"category":  "updated",
		"is_active": false,
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("PUT", "/products/"+strconv.Itoa(int(product.ID)), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(product.ID))})
	w := httptest.NewRecorder()

	handlers.ProductHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.Product
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %q", result.Name)
	}
	if result.SKU != "SKU-UPD" {
		t.Errorf("Expected SKU 'SKU-UPD', got %q", result.SKU)
	}
	if result.Category != "updated" {
		t.Errorf("Expected category 'updated', got %q", result.Category)
	}
}

func TestDeleteProduct(t *testing.T) {
	testDB := setupTestDB(t)

	product := models.Product{Name: "Delete Me", SKU: "SKU-DEL", Category: "test", IsActive: true}
	testDB.Create(&product)

	req := httptest.NewRequest("DELETE", "/products/"+strconv.Itoa(int(product.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(product.ID))})
	w := httptest.NewRecorder()

	handlers.ProductHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify product is gone
	var count int64
	testDB.Model(&models.Product{}).Where("id = ?", product.ID).Count(&count)
	if count != 0 {
		t.Errorf("Expected product to be deleted, but found %d", count)
	}
}

// ── Services CRUD Tests ──────────────────────────────────────────────────────

func TestGetAllServices(t *testing.T) {
	testDB := setupTestDB(t)

	testDB.Create(&models.Service{Code: "SVC-1", Name: "Service A", IsActive: true})
	testDB.Create(&models.Service{Code: "SVC-2", Name: "Service B", IsActive: true})

	req := httptest.NewRequest("GET", "/services", nil)
	w := httptest.NewRecorder()

	handlers.ServicesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var services []models.Service
	if err := json.Unmarshal(w.Body.Bytes(), &services); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(services))
	}
}

func TestCreateService(t *testing.T) {
	setupTestDB(t)

	payload := map[string]interface{}{
		"code": "SVC-NEW",
		"name": "New Service",
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/services", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handlers.ServicesHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var service models.Service
	if err := json.Unmarshal(w.Body.Bytes(), &service); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if service.Code != "SVC-NEW" {
		t.Errorf("Expected code 'SVC-NEW', got %q", service.Code)
	}
	if service.Name != "New Service" {
		t.Errorf("Expected name 'New Service', got %q", service.Name)
	}
}

func TestCreateService_NonAdmin(t *testing.T) {
	setupTestDB(t)

	payload := map[string]interface{}{
		"code": "SVC-NOAUTH",
		"name": "Unauthorized Service",
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/services", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "worker")
	w := httptest.NewRecorder()

	handlers.ServicesHandler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestCreateService_MissingFields(t *testing.T) {
	setupTestDB(t)

	payload := map[string]interface{}{
		"code": "",
		"name": "",
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/services", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handlers.ServicesHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestGetServiceByID(t *testing.T) {
	testDB := setupTestDB(t)

	service := models.Service{Code: "SVC-GET", Name: "Get Service", IsActive: true}
	testDB.Create(&service)

	req := httptest.NewRequest("GET", "/services/"+strconv.Itoa(int(service.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(service.ID))})
	w := httptest.NewRecorder()

	handlers.ServiceHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.Service
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.Code != "SVC-GET" {
		t.Errorf("Expected code 'SVC-GET', got %q", result.Code)
	}
	if result.Name != "Get Service" {
		t.Errorf("Expected name 'Get Service', got %q", result.Name)
	}
}

func TestUpdateService(t *testing.T) {
	testDB := setupTestDB(t)

	service := models.Service{Code: "SVC-OLD", Name: "Old Service", IsActive: true}
	testDB.Create(&service)

	payload := map[string]interface{}{
		"code":      "SVC-UPD",
		"name":      "Updated Service",
		"is_active": false,
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("PUT", "/services/"+strconv.Itoa(int(service.ID)), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(service.ID))})
	w := httptest.NewRecorder()

	handlers.ServiceHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.Service
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.Code != "SVC-UPD" {
		t.Errorf("Expected code 'SVC-UPD', got %q", result.Code)
	}
	if result.Name != "Updated Service" {
		t.Errorf("Expected name 'Updated Service', got %q", result.Name)
	}
}

func TestDeleteService(t *testing.T) {
	testDB := setupTestDB(t)

	service := models.Service{Code: "SVC-DEL", Name: "Delete Service", IsActive: true}
	testDB.Create(&service)

	req := httptest.NewRequest("DELETE", "/services/"+strconv.Itoa(int(service.ID)), nil)
	req.Header.Set("X-User-Role", "admin")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(service.ID))})
	w := httptest.NewRecorder()

	handlers.ServiceHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify service is gone
	var count int64
	testDB.Model(&models.Service{}).Where("id = ?", service.ID).Count(&count)
	if count != 0 {
		t.Errorf("Expected service to be deleted, but found %d", count)
	}
}

// ── Signals CRUD Tests ───────────────────────────────────────────────────────

func TestGetAllSignals(t *testing.T) {
	testDB := setupTestDB(t)

	device := createTestDevice(t, testDB, "sig-device", "sig-token")
	testDB.Create(&models.Signal{DeviceID: device.ID, Name: "Signal A", SignalType: "analogic", Direction: "input", IsActive: true})
	testDB.Create(&models.Signal{DeviceID: device.ID, Name: "Signal B", SignalType: "digital", Direction: "output", IsActive: true})

	req := httptest.NewRequest("GET", "/signals", nil)
	w := httptest.NewRecorder()

	handlers.SignalsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var signals []models.Signal
	if err := json.Unmarshal(w.Body.Bytes(), &signals); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(signals) != 2 {
		t.Errorf("Expected 2 signals, got %d", len(signals))
	}
}

func TestCreateSignal(t *testing.T) {
	testDB := setupTestDB(t)

	device := createTestDevice(t, testDB, "sig-create-dev", "sig-create-token")

	payload := map[string]interface{}{
		"device_id":   device.ID,
		"name":        "Temperature",
		"signal_type": "analogic",
		"direction":   "input",
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/signals", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.SignalsHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var signal models.Signal
	if err := json.Unmarshal(w.Body.Bytes(), &signal); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if signal.Name != "Temperature" {
		t.Errorf("Expected name 'Temperature', got %q", signal.Name)
	}
	if signal.SignalType != "analogic" {
		t.Errorf("Expected signal_type 'analogic', got %q", signal.SignalType)
	}
	if signal.Direction != "input" {
		t.Errorf("Expected direction 'input', got %q", signal.Direction)
	}
	if signal.DeviceID != device.ID {
		t.Errorf("Expected device_id %d, got %d", device.ID, signal.DeviceID)
	}
}

func TestCreateSignal_MissingDeviceID(t *testing.T) {
	setupTestDB(t)

	payload := map[string]interface{}{
		"name":        "No Device Signal",
		"signal_type": "analogic",
		"direction":   "input",
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/signals", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.SignalsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestCreateSignal_InvalidDevice(t *testing.T) {
	setupTestDB(t)

	payload := map[string]interface{}{
		"device_id":   9999,
		"name":        "Invalid Device Signal",
		"signal_type": "analogic",
		"direction":   "input",
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/signals", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.SignalsHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestGetSignalByID(t *testing.T) {
	testDB := setupTestDB(t)

	device := createTestDevice(t, testDB, "sig-get-dev", "sig-get-token")
	signal := models.Signal{DeviceID: device.ID, Name: "Get Signal", SignalType: "analogic", Direction: "input", IsActive: true}
	testDB.Create(&signal)

	req := httptest.NewRequest("GET", "/signals/"+strconv.Itoa(int(signal.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(signal.ID))})
	w := httptest.NewRecorder()

	handlers.SignalHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.Signal
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.Name != "Get Signal" {
		t.Errorf("Expected name 'Get Signal', got %q", result.Name)
	}
}

func TestUpdateSignal(t *testing.T) {
	testDB := setupTestDB(t)

	device := createTestDevice(t, testDB, "sig-upd-dev", "sig-upd-token")
	signal := models.Signal{DeviceID: device.ID, Name: "Old Signal", SignalType: "analogic", Direction: "input", Unit: "C", IsActive: true}
	testDB.Create(&signal)

	payload := map[string]interface{}{
		"name":      "Updated Signal",
		"unit":      "F",
		"is_active": true,
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("PUT", "/signals/"+strconv.Itoa(int(signal.ID)), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(signal.ID))})
	w := httptest.NewRecorder()

	handlers.SignalHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.Signal
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.Name != "Updated Signal" {
		t.Errorf("Expected name 'Updated Signal', got %q", result.Name)
	}
	if result.Unit != "F" {
		t.Errorf("Expected unit 'F', got %q", result.Unit)
	}
}

func TestDeleteSignal(t *testing.T) {
	testDB := setupTestDB(t)

	device := createTestDevice(t, testDB, "sig-del-dev", "sig-del-token")
	signal := models.Signal{DeviceID: device.ID, Name: "Delete Signal", SignalType: "analogic", Direction: "input", IsActive: true}
	testDB.Create(&signal)

	req := httptest.NewRequest("DELETE", "/signals/"+strconv.Itoa(int(signal.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(signal.ID))})
	w := httptest.NewRecorder()

	handlers.SignalHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify signal is gone
	var count int64
	testDB.Model(&models.Signal{}).Where("id = ?", signal.ID).Count(&count)
	if count != 0 {
		t.Errorf("Expected signal to be deleted, but found %d", count)
	}
}

// ── SignalValues CRUD Tests ──────────────────────────────────────────────────

func TestCreateSignalValue_Analogic(t *testing.T) {
	testDB := setupTestDB(t)

	device := createTestDevice(t, testDB, "sv-analog-dev", "sv-analog-token")
	signal := models.Signal{DeviceID: device.ID, Name: "Temp", SignalType: "analogic", Direction: "input", IsActive: true}
	testDB.Create(&signal)

	val := 23.5
	payload := map[string]interface{}{
		"signal_id": signal.ID,
		"value":     val,
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/signal-values", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.SignalValuesHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.SignalValue
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.Value == nil || *result.Value != val {
		t.Errorf("Expected value %f, got %v", val, result.Value)
	}
	if result.SignalID != signal.ID {
		t.Errorf("Expected signal_id %d, got %d", signal.ID, result.SignalID)
	}
}

func TestCreateSignalValue_Digital(t *testing.T) {
	testDB := setupTestDB(t)

	device := createTestDevice(t, testDB, "sv-digital-dev", "sv-digital-token")
	signal := models.Signal{DeviceID: device.ID, Name: "Switch", SignalType: "digital", Direction: "input", IsActive: true}
	testDB.Create(&signal)

	payload := map[string]interface{}{
		"signal_id":     signal.ID,
		"digital_value": true,
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/signal-values", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.SignalValuesHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.SignalValue
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.DigitalValue == nil || *result.DigitalValue != true {
		t.Errorf("Expected digital_value true, got %v", result.DigitalValue)
	}
}

func TestCreateSignalValue_MissingSignalID(t *testing.T) {
	setupTestDB(t)

	payload := map[string]interface{}{
		"value": 10.0,
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/signal-values", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.SignalValuesHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestGetSignalValueByID(t *testing.T) {
	testDB := setupTestDB(t)

	device := createTestDevice(t, testDB, "sv-get-dev", "sv-get-token")
	signal := models.Signal{DeviceID: device.ID, Name: "GetVal", SignalType: "analogic", Direction: "input", IsActive: true}
	testDB.Create(&signal)

	val := 42.0
	sv := models.SignalValue{SignalID: signal.ID, Value: &val}
	testDB.Create(&sv)

	req := httptest.NewRequest("GET", "/signal-values/"+strconv.Itoa(int(sv.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(sv.ID))})
	w := httptest.NewRecorder()

	handlers.SignalValueHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.SignalValue
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.Value == nil || *result.Value != val {
		t.Errorf("Expected value %f, got %v", val, result.Value)
	}
}

func TestDeleteSignalValue(t *testing.T) {
	testDB := setupTestDB(t)

	device := createTestDevice(t, testDB, "sv-del-dev", "sv-del-token")
	signal := models.Signal{DeviceID: device.ID, Name: "DelVal", SignalType: "analogic", Direction: "input", IsActive: true}
	testDB.Create(&signal)

	val := 99.0
	sv := models.SignalValue{SignalID: signal.ID, Value: &val}
	testDB.Create(&sv)

	req := httptest.NewRequest("DELETE", "/signal-values/"+strconv.Itoa(int(sv.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(sv.ID))})
	w := httptest.NewRecorder()

	handlers.SignalValueHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify signal value is gone
	var count int64
	testDB.Model(&models.SignalValue{}).Where("id = ?", sv.ID).Count(&count)
	if count != 0 {
		t.Errorf("Expected signal value to be deleted, but found %d", count)
	}
}

// ── Customers CRUD Tests ─────────────────────────────────────────────────────

func TestGetAllCustomers(t *testing.T) {
	testDB := setupTestDB(t)

	testDB.Create(&models.Customer{Name: "Customer A", Phone: "111-1111"})
	testDB.Create(&models.Customer{Name: "Customer B", Phone: "222-2222"})

	req := httptest.NewRequest("GET", "/customers", nil)
	w := httptest.NewRecorder()

	handlers.CustomersHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var customers []models.Customer
	if err := json.Unmarshal(w.Body.Bytes(), &customers); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(customers) != 2 {
		t.Errorf("Expected 2 customers, got %d", len(customers))
	}
}

func TestCreateCustomer(t *testing.T) {
	setupTestDB(t)

	payload := map[string]interface{}{
		"name":  "New Customer",
		"phone": "333-3333",
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/customers", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.CustomersHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var customer models.Customer
	if err := json.Unmarshal(w.Body.Bytes(), &customer); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if customer.Name != "New Customer" {
		t.Errorf("Expected name 'New Customer', got %q", customer.Name)
	}
	if customer.Phone != "333-3333" {
		t.Errorf("Expected phone '333-3333', got %q", customer.Phone)
	}
}

func TestCreateCustomer_MissingName(t *testing.T) {
	setupTestDB(t)

	payload := map[string]interface{}{
		"phone": "444-4444",
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/customers", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.CustomersHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestGetCustomerByID(t *testing.T) {
	testDB := setupTestDB(t)

	customer := models.Customer{Name: "Lookup Customer", Phone: "555-5555"}
	testDB.Create(&customer)

	req := httptest.NewRequest("GET", "/customers/"+strconv.Itoa(int(customer.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(customer.ID))})
	w := httptest.NewRecorder()

	handlers.CustomerByIDHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.Customer
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.Name != "Lookup Customer" {
		t.Errorf("Expected name 'Lookup Customer', got %q", result.Name)
	}
	if result.Phone != "555-5555" {
		t.Errorf("Expected phone '555-5555', got %q", result.Phone)
	}
}

func TestUpdateCustomer(t *testing.T) {
	testDB := setupTestDB(t)

	customer := models.Customer{Name: "Old Customer", Phone: "666-6666"}
	testDB.Create(&customer)

	payload := map[string]interface{}{
		"name":  "Updated Customer",
		"phone": "777-7777",
	}
	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("PUT", "/customers/"+strconv.Itoa(int(customer.ID)), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(customer.ID))})
	w := httptest.NewRecorder()

	handlers.CustomerByIDHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var result models.Customer
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result.Name != "Updated Customer" {
		t.Errorf("Expected name 'Updated Customer', got %q", result.Name)
	}
	if result.Phone != "777-7777" {
		t.Errorf("Expected phone '777-7777', got %q", result.Phone)
	}
}

func TestDeleteCustomer(t *testing.T) {
	testDB := setupTestDB(t)

	customer := models.Customer{Name: "Delete Customer", Phone: "888-8888"}
	testDB.Create(&customer)

	req := httptest.NewRequest("DELETE", "/customers/"+strconv.Itoa(int(customer.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(customer.ID))})
	w := httptest.NewRecorder()

	handlers.CustomerByIDHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify customer is gone
	var count int64
	testDB.Model(&models.Customer{}).Where("id = ?", customer.ID).Count(&count)
	if count != 0 {
		t.Errorf("Expected customer to be deleted, but found %d", count)
	}
}

// ── Image MIME Type Validation Tests ────────────────────────────────────────

func TestImageUpload_InvalidMIME(t *testing.T) {
	testDB := setupTestDB(t)

	product := models.Product{Name: "MIME Test", SKU: "MIME-1", Category: "test", IsActive: true}
	testDB.Create(&product)

	// Plain text bytes — http.DetectContentType returns "text/plain"
	textData := []byte("this is not an image, just plain text content")
	req := createMultipartImageRequest(t, "PUT", "/products/"+strconv.Itoa(int(product.ID))+"/image", textData)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(product.ID))})
	w := httptest.NewRecorder()

	handler := handlers.ImageUploadHandler("products")
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for non-image MIME type, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestImageUpload_ValidJPEG(t *testing.T) {
	testDB := setupTestDB(t)

	product := models.Product{Name: "JPEG Test", SKU: "JPEG-1", Category: "test", IsActive: true}
	testDB.Create(&product)

	// Minimal JPEG (JFIF header) — http.DetectContentType returns "image/jpeg"
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00}
	req := createMultipartImageRequest(t, "PUT", "/products/"+strconv.Itoa(int(product.ID))+"/image", jpegData)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(product.ID))})
	w := httptest.NewRecorder()

	handler := handlers.ImageUploadHandler("products")
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for valid JPEG, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify the stored image_type contains "image/jpeg"
	var saved models.Product
	testDB.First(&saved, product.ID)
	if !strings.Contains(saved.ImageType, "image/jpeg") {
		t.Errorf("Expected image_type to contain 'image/jpeg', got %q", saved.ImageType)
	}
}

func TestImageDownload_NoImage(t *testing.T) {
	testDB := setupTestDB(t)

	// Product WITHOUT image data
	product := models.Product{Name: "No Image", SKU: "NOIMG-1", Category: "test", IsActive: true}
	testDB.Create(&product)

	req := httptest.NewRequest("GET", "/products/"+strconv.Itoa(int(product.ID))+"/image", nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(product.ID))})
	w := httptest.NewRecorder()

	handler := handlers.ImageDownloadHandler("products")
	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for product without image, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestImageUpload_InvalidEntity(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest("PUT", "/invalid_entity/1/image", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	handler := handlers.ImageUploadHandler("invalid_entity")
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid entity, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// ── Auth Middleware Tests ────────────────────────────────────────────────────

func TestRequireUserAuth_ValidToken(t *testing.T) {
	testDB := setupTestDB(t)
	user := createTestUser(t, testDB, "Auth User", "authuser@test.com", "pass123", "worker")

	token, err := auth.GenerateJWT(user.ID, user.Email, user.Type)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	called := false
	var capturedUserID string
	inner := func(w http.ResponseWriter, r *http.Request) {
		called = true
		capturedUserID = r.Header.Get("X-User-ID")
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := auth.RequireUserAuth(inner)
	handler(w, req)

	if !called {
		t.Error("Expected inner handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if capturedUserID != strconv.Itoa(int(user.ID)) {
		t.Errorf("Expected X-User-ID %d, got %q", user.ID, capturedUserID)
	}
}

func TestRequireUserAuth_NoToken(t *testing.T) {
	setupTestDB(t)

	inner := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler := auth.RequireUserAuth(inner)
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRequireUserAuth_InvalidToken(t *testing.T) {
	setupTestDB(t)

	inner := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-jwt-token")
	w := httptest.NewRecorder()

	handler := auth.RequireUserAuth(inner)
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRequireAdmin_AdminUser(t *testing.T) {
	testDB := setupTestDB(t)
	admin := createTestUser(t, testDB, "Admin Auth", "adminauth@test.com", "pass123", "admin")

	token, err := auth.GenerateJWT(admin.ID, admin.Email, admin.Type)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	called := false
	inner := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := auth.RequireAdmin(inner)
	handler(w, req)

	if !called {
		t.Error("Expected inner handler to be called for admin user")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRequireAdmin_WorkerUser(t *testing.T) {
	testDB := setupTestDB(t)
	worker := createTestUser(t, testDB, "Worker Auth", "workerauth@test.com", "pass123", "worker")

	token, err := auth.GenerateJWT(worker.ID, worker.Email, worker.Type)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	inner := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := auth.RequireAdmin(inner)
	handler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for worker user, got %d", w.Code)
	}
}

func TestRequireDeviceAuth_ValidToken(t *testing.T) {
	testDB := setupTestDB(t)
	device := createTestDevice(t, testDB, "auth-device", "device-auth-token-123")

	called := false
	var capturedDeviceID string
	inner := func(w http.ResponseWriter, r *http.Request) {
		called = true
		capturedDeviceID = r.Header.Get("X-Device-ID")
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer device-auth-token-123")
	w := httptest.NewRecorder()

	handler := auth.RequireDeviceAuth(inner)
	handler(w, req)

	if !called {
		t.Error("Expected inner handler to be called for valid device token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if capturedDeviceID != strconv.Itoa(int(device.ID)) {
		t.Errorf("Expected X-Device-ID %d, got %q", device.ID, capturedDeviceID)
	}
}

func TestRequireDeviceAuth_InvalidToken(t *testing.T) {
	setupTestDB(t)

	inner := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer nonexistent-token")
	w := httptest.NewRecorder()

	handler := auth.RequireDeviceAuth(inner)
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRequireAnyAuth_UserJWT(t *testing.T) {
	testDB := setupTestDB(t)
	user := createTestUser(t, testDB, "Any Auth User", "anyauth@test.com", "pass123", "worker")

	token, err := auth.GenerateJWT(user.ID, user.Email, user.Type)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	called := false
	var capturedAuthType string
	inner := func(w http.ResponseWriter, r *http.Request) {
		called = true
		capturedAuthType = r.Header.Get("X-Auth-Type")
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := auth.RequireAnyAuth(inner)
	handler(w, req)

	if !called {
		t.Error("Expected inner handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if capturedAuthType != "user" {
		t.Errorf("Expected X-Auth-Type 'user', got %q", capturedAuthType)
	}
}

func TestRequireAnyAuth_DeviceToken(t *testing.T) {
	testDB := setupTestDB(t)
	createTestDevice(t, testDB, "anyauth-device", "anyauth-device-token")

	called := false
	var capturedAuthType string
	inner := func(w http.ResponseWriter, r *http.Request) {
		called = true
		capturedAuthType = r.Header.Get("X-Auth-Type")
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer anyauth-device-token")
	w := httptest.NewRecorder()

	handler := auth.RequireAnyAuth(inner)
	handler(w, req)

	if !called {
		t.Error("Expected inner handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if capturedAuthType != "device" {
		t.Errorf("Expected X-Auth-Type 'device', got %q", capturedAuthType)
	}
}

func TestValidateJWT_MalformedToken(t *testing.T) {
	_, err := auth.ValidateJWT("invalid.token.here")
	if err == nil {
		t.Error("Expected error for malformed token, got nil")
	}
}
