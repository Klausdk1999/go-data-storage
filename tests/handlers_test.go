package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	jsonData, _ := json.Marshal(body)
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
