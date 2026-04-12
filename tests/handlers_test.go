package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
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

	// PUT new preferences (should merge, overwriting theme but keeping language)
	newPrefs := map[string]interface{}{"theme": "dark", "sidebar_collapsed": true}
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

	// Verify in DB
	var updated models.User
	testDB.First(&updated, user.ID)

	if updated.Preferences["theme"] != "dark" {
		t.Errorf("Expected theme 'dark', got %v", updated.Preferences["theme"])
	}
	if updated.Preferences["language"] != "en" {
		t.Errorf("Expected language 'en' (preserved from merge), got %v", updated.Preferences["language"])
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
