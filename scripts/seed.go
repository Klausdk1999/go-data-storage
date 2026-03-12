package main

import (
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"data-storage/internal/auth"
	"data-storage/internal/db"
	"data-storage/internal/models"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize database
	dbConfig := db.LoadConfigFromEnv()
	database, err := db.InitDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	log.Println("Starting database seeding...")

	// Clear existing data (child tables first to respect FK constraints)
	log.Println("Clearing existing data...")
	database.Exec("TRUNCATE TABLE time_entries CASCADE")
	database.Exec("TRUNCATE TABLE services CASCADE")
	database.Exec("TRUNCATE TABLE stock_movements CASCADE")
	database.Exec("TRUNCATE TABLE production_orders CASCADE")
	database.Exec("TRUNCATE TABLE customers CASCADE")
	database.Exec("TRUNCATE TABLE bill_of_materials CASCADE")
	database.Exec("TRUNCATE TABLE raw_materials CASCADE")
	database.Exec("TRUNCATE TABLE products CASCADE")
	database.Exec("TRUNCATE TABLE signal_values CASCADE")
	database.Exec("TRUNCATE TABLE signals CASCADE")
	database.Exec("TRUNCATE TABLE devices CASCADE")
	database.Exec("TRUNCATE TABLE users CASCADE")

	// ========================================
	// 1. Create users
	// ========================================
	adminUser := models.User{
		Name:     "Admin User",
		Email:    "admin@test.com",
		Type:     "admin",
		Rfid:     "RFID001",
		IsActive: true,
	}
	err = adminUser.SetPassword("admin123")
	if err != nil {
		log.Fatalf("Failed to set password: %v", err)
	}
	result := database.Create(&adminUser)
	if result.Error != nil {
		log.Fatalf("Failed to create admin user: %v", result.Error)
	}
	log.Printf("Created user: %s (ID: %d, Email: %s)", adminUser.Name, adminUser.ID, adminUser.Email)

	workerUser := models.User{
		Name:     "Maria Santos",
		Email:    "worker@test.com",
		Type:     "worker",
		Rfid:     "RFID002",
		IsActive: true,
	}
	err = workerUser.SetPassword("worker123")
	if err != nil {
		log.Fatalf("Failed to set password: %v", err)
	}
	result = database.Create(&workerUser)
	if result.Error != nil {
		log.Fatalf("Failed to create worker user: %v", result.Error)
	}
	log.Printf("Created user: %s (ID: %d, Email: %s)", workerUser.Name, workerUser.ID, workerUser.Email)

	// ========================================
	// 2. Create devices (linked to admin user)
	// ========================================
	devices := []models.Device{
		{
			Name:        "Temperature Sensor 1",
			Description: "Main temperature sensor in living room",
			DeviceType:  "sensor",
			Location:    "Living Room",
			UserID:      &adminUser.ID,
			IsActive:    true,
		},
		{
			Name:        "Humidity Sensor 1",
			Description: "Humidity sensor in kitchen",
			DeviceType:  "sensor",
			Location:    "Kitchen",
			UserID:      &adminUser.ID,
			IsActive:    true,
		},
		{
			Name:        "Smart Light Switch",
			Description: "Smart switch controlling living room lights",
			DeviceType:  "actuator",
			Location:    "Living Room",
			UserID:      &adminUser.ID,
			IsActive:    true,
		},
	}

	for i := range devices {
		authToken, err := auth.GenerateDeviceToken()
		if err != nil {
			log.Fatalf("Failed to generate device token: %v", err)
		}
		devices[i].AuthToken = authToken

		result := database.Create(&devices[i])
		if result.Error != nil {
			log.Fatalf("Failed to create device: %v", result.Error)
		}
		log.Printf("Created device: %s (ID: %d, Token: %s)", devices[i].Name, devices[i].ID, devices[i].AuthToken)
	}

	// ========================================
	// 3. Create signals for each device
	// ========================================
	now := time.Now()

	// Signals for Temperature Sensor 1
	tempSensor := devices[0]
	temperatureSignal := models.Signal{
		DeviceID:    tempSensor.ID,
		Name:        "Temperature",
		SignalType:  "analogic",
		Direction:   "input",
		SensorName:  "DS18B20",
		Description: "Temperature reading in Celsius",
		Unit:        "°C",
		MinValue:    func() *float64 { v := -20.0; return &v }(),
		MaxValue:    func() *float64 { v := 60.0; return &v }(),
		IsActive:    true,
		Metadata:    models.JSONB{"calibration_date": "2024-01-15", "accuracy": "±0.5°C"},
	}
	database.Create(&temperatureSignal)
	log.Printf("Created signal: %s (ID: %d)", temperatureSignal.Name, temperatureSignal.ID)

	// Signals for Humidity Sensor 1
	humiditySensor := devices[1]
	humiditySignal := models.Signal{
		DeviceID:    humiditySensor.ID,
		Name:        "Humidity",
		SignalType:  "analogic",
		Direction:   "input",
		SensorName:  "DHT22",
		Description: "Relative humidity percentage",
		Unit:        "%",
		MinValue:    func() *float64 { v := 0.0; return &v }(),
		MaxValue:    func() *float64 { v := 100.0; return &v }(),
		IsActive:    true,
		Metadata:    models.JSONB{"model": "DHT22", "resolution": "0.1%"},
	}
	database.Create(&humiditySignal)
	log.Printf("Created signal: %s (ID: %d)", humiditySignal.Name, humiditySignal.ID)

	pressureSignal := models.Signal{
		DeviceID:    humiditySensor.ID,
		Name:        "Pressure",
		SignalType:  "analogic",
		Direction:   "input",
		SensorName:  "BMP280",
		Description: "Atmospheric pressure",
		Unit:        "hPa",
		MinValue:    func() *float64 { v := 300.0; return &v }(),
		MaxValue:    func() *float64 { v := 1100.0; return &v }(),
		IsActive:    true,
		Metadata:    models.JSONB{"sensor": "BMP280", "range": "300-1100 hPa"},
	}
	database.Create(&pressureSignal)
	log.Printf("Created signal: %s (ID: %d)", pressureSignal.Name, pressureSignal.ID)

	// Signals for Smart Light Switch
	lightSwitch := devices[2]
	lightSwitchSignal := models.Signal{
		DeviceID:    lightSwitch.ID,
		Name:        "Light State",
		SignalType:  "digital",
		Direction:   "output",
		Description: "ON/OFF state of the light",
		IsActive:    true,
		Metadata:    models.JSONB{"default_state": "off"},
	}
	database.Create(&lightSwitchSignal)
	log.Printf("Created signal: %s (ID: %d)", lightSwitchSignal.Name, lightSwitchSignal.ID)

	motionSensorSignal := models.Signal{
		DeviceID:    lightSwitch.ID,
		Name:        "Motion Detected",
		SignalType:  "digital",
		Direction:   "input",
		SensorName:  "PIR Sensor",
		Description: "Motion detection input",
		IsActive:    true,
		Metadata:    models.JSONB{"sensor_type": "PIR", "range": "5m"},
	}
	database.Create(&motionSensorSignal)
	log.Printf("Created signal: %s (ID: %d)", motionSensorSignal.Name, motionSensorSignal.ID)

	// ========================================
	// 4. Create signal values (historical data)
	// ========================================
	log.Println("\nCreating signal values...")

	// Temperature values (last 24 hours, every hour)
	rand.Seed(time.Now().UnixNano())
	baseTime := now.Add(-24 * time.Hour)
	for i := 0; i < 24; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		tempValue := 20.0 + (rand.Float64() * 7.0) + (math.Sin(float64(i)/3.0) * 2.0)
		value := models.SignalValue{
			SignalID:  temperatureSignal.ID,
			UserID:    &adminUser.ID,
			Timestamp: timestamp,
			Value:     &tempValue,
			Metadata:  models.JSONB{"source": "automated"},
		}
		database.Create(&value)
	}

	// Humidity values (last 12 hours, every 30 minutes)
	baseTime = now.Add(-12 * time.Hour)
	for i := 0; i < 24; i++ {
		timestamp := baseTime.Add(time.Duration(i) * 30 * time.Minute)
		humidityValue := 45.0 + (rand.Float64() * 15.0)
		value := models.SignalValue{
			SignalID:  humiditySignal.ID,
			UserID:    &adminUser.ID,
			Timestamp: timestamp,
			Value:     &humidityValue,
			Metadata:  models.JSONB{"source": "automated"},
		}
		database.Create(&value)
	}

	// Pressure values (last 6 hours, every 15 minutes)
	baseTime = now.Add(-6 * time.Hour)
	for i := 0; i < 24; i++ {
		timestamp := baseTime.Add(time.Duration(i) * 15 * time.Minute)
		pressureValue := 1013.25 + (rand.Float64() * 20.0) - 10.0
		value := models.SignalValue{
			SignalID:  pressureSignal.ID,
			UserID:    &adminUser.ID,
			Timestamp: timestamp,
			Value:     &pressureValue,
			Metadata:  models.JSONB{"source": "automated"},
		}
		database.Create(&value)
	}

	// Light state values (digital - last 24 hours, state changes)
	baseTime = now.Add(-24 * time.Hour)
	lightState := false
	for i := 0; i < 10; i++ {
		timestamp := baseTime.Add(time.Duration(i*2+rand.Intn(3)) * time.Hour)
		lightState = !lightState
		value := models.SignalValue{
			SignalID:     lightSwitchSignal.ID,
			UserID:       &adminUser.ID,
			Timestamp:    timestamp,
			DigitalValue: &lightState,
			Metadata:     models.JSONB{"source": "manual"},
		}
		database.Create(&value)
	}

	// Motion sensor values (digital - last 12 hours, random detections)
	baseTime = now.Add(-12 * time.Hour)
	for i := 0; i < 15; i++ {
		timestamp := baseTime.Add(time.Duration(rand.Intn(720)) * time.Minute)
		motionDetected := rand.Float64() > 0.7
		value := models.SignalValue{
			SignalID:     motionSensorSignal.ID,
			UserID:       &adminUser.ID,
			Timestamp:    timestamp,
			DigitalValue: &motionDetected,
			Metadata:     models.JSONB{"source": "sensor"},
		}
		database.Create(&value)
	}

	// ========================================
	// 5. Create products
	// ========================================
	log.Println("\nCreating products...")

	steelBracket := models.Product{
		Name:     "Steel Bracket",
		SKU:      "PROD-001",
		Unit:     "unit",
		Category: "structural",
		IsActive: true,
	}
	database.Create(&steelBracket)
	log.Printf("Created product: %s (ID: %d)", steelBracket.Name, steelBracket.ID)

	aluminumFrame := models.Product{
		Name:     "Aluminum Frame",
		SKU:      "PROD-002",
		Unit:     "unit",
		Category: "structural",
		IsActive: true,
	}
	database.Create(&aluminumFrame)
	log.Printf("Created product: %s (ID: %d)", aluminumFrame.Name, aluminumFrame.ID)

	// ========================================
	// 6. Create raw materials
	// ========================================
	log.Println("\nCreating raw materials...")

	steelSheet := models.RawMaterial{
		Name:          "Steel Sheet 2mm",
		SKU:           "RM-001",
		Unit:          "kg",
		StockQuantity: 500,
		MinStock:      func() *float64 { v := 50.0; return &v }(),
		Category:      "metal",
		IsActive:      true,
	}
	database.Create(&steelSheet)
	log.Printf("Created raw material: %s (ID: %d)", steelSheet.Name, steelSheet.ID)

	aluminumBar := models.RawMaterial{
		Name:          "Aluminum Bar 10mm",
		SKU:           "RM-002",
		Unit:          "kg",
		StockQuantity: 200,
		MinStock:      func() *float64 { v := 30.0; return &v }(),
		Category:      "metal",
		IsActive:      true,
	}
	database.Create(&aluminumBar)
	log.Printf("Created raw material: %s (ID: %d)", aluminumBar.Name, aluminumBar.ID)

	boltsM8 := models.RawMaterial{
		Name:          "Bolts M8x30",
		SKU:           "RM-003",
		Unit:          "unit",
		StockQuantity: 5000,
		MinStock:      func() *float64 { v := 500.0; return &v }(),
		Category:      "fastener",
		IsActive:      true,
	}
	database.Create(&boltsM8)
	log.Printf("Created raw material: %s (ID: %d)", boltsM8.Name, boltsM8.ID)

	// ========================================
	// 7. Create BOM entries
	// ========================================
	log.Println("\nCreating bill of materials...")

	// Steel Bracket BOM
	database.Create(&models.BillOfMaterials{
		ProductID:     steelBracket.ID,
		RawMaterialID: steelSheet.ID,
		Quantity:      2.5,
	})
	database.Create(&models.BillOfMaterials{
		ProductID:     steelBracket.ID,
		RawMaterialID: boltsM8.ID,
		Quantity:      8,
	})
	log.Printf("Created BOM for Steel Bracket: 2.5kg Steel Sheet + 8 Bolts M8")

	// Aluminum Frame BOM
	database.Create(&models.BillOfMaterials{
		ProductID:     aluminumFrame.ID,
		RawMaterialID: aluminumBar.ID,
		Quantity:      4.0,
	})
	database.Create(&models.BillOfMaterials{
		ProductID:     aluminumFrame.ID,
		RawMaterialID: boltsM8.ID,
		Quantity:      12,
	})
	log.Printf("Created BOM for Aluminum Frame: 4.0kg Aluminum Bar + 12 Bolts M8")

	// ========================================
	// 8. Create customers
	// ========================================
	log.Println("\nCreating customers...")

	acmeCorp := models.Customer{Name: "Acme Corp"}
	database.Create(&acmeCorp)
	log.Printf("Created customer: %s (ID: %d)", acmeCorp.Name, acmeCorp.ID)

	buildRight := models.Customer{Name: "BuildRight Inc"}
	database.Create(&buildRight)
	log.Printf("Created customer: %s (ID: %d)", buildRight.Name, buildRight.ID)

	// ========================================
	// 9. Create services
	// ========================================
	log.Println("\nCreating services...")

	svcAssembly := models.Service{
		Code:        "SVC-001",
		Name:        "Assembly",
		Description: "Product assembly and fabrication",
		IsActive:    true,
	}
	database.Create(&svcAssembly)
	log.Printf("Created service: %s (ID: %d)", svcAssembly.Name, svcAssembly.ID)

	svcQC := models.Service{
		Code:        "SVC-002",
		Name:        "Quality Control",
		Description: "Quality inspection and testing",
		IsActive:    true,
	}
	database.Create(&svcQC)
	log.Printf("Created service: %s (ID: %d)", svcQC.Name, svcQC.ID)

	svcPackaging := models.Service{
		Code:        "SVC-003",
		Name:        "Packaging",
		Description: "Product packaging and preparation",
		IsActive:    true,
	}
	database.Create(&svcPackaging)
	log.Printf("Created service: %s (ID: %d)", svcPackaging.Name, svcPackaging.ID)

	// ========================================
	// 10. Create production orders
	// ========================================
	log.Println("\nCreating production orders...")

	twoDaysAgo := now.Add(-48 * time.Hour)
	orderSteelBracket := models.ProductionOrder{
		ProductID:  steelBracket.ID,
		CustomerID: &acmeCorp.ID,
		Quantity:   100,
		Status:     "in_progress",
		Priority:   1,
		StartedAt:  &twoDaysAgo,
	}
	database.Create(&orderSteelBracket)
	log.Printf("Created production order: Steel Bracket x100 for Acme Corp (ID: %d, status: in_progress)", orderSteelBracket.ID)

	orderAluminumFrame := models.ProductionOrder{
		ProductID:  aluminumFrame.ID,
		CustomerID: &buildRight.ID,
		Quantity:   50,
		Status:     "planned",
		Priority:   2,
	}
	database.Create(&orderAluminumFrame)
	log.Printf("Created production order: Aluminum Frame x50 for BuildRight Inc (ID: %d, status: planned)", orderAluminumFrame.ID)

	// ========================================
	// 11. Create time entries
	// ========================================
	log.Println("\nCreating time entries...")

	yesterday := now.Add(-24 * time.Hour).Format("2006-01-02")
	today := now.Format("2006-01-02")

	database.Create(&models.TimeEntry{
		UserID:            workerUser.ID,
		ProductionOrderID: orderSteelBracket.ID,
		ServiceID:         svcAssembly.ID,
		Day:               yesterday,
		StartTime:         "08:00",
		EndTime:           "12:00",
		Observations:      "Morning shift assembly",
	})
	log.Printf("Created time entry: %s, Assembly 08:00-12:00", yesterday)

	database.Create(&models.TimeEntry{
		UserID:            workerUser.ID,
		ProductionOrderID: orderSteelBracket.ID,
		ServiceID:         svcQC.ID,
		Day:               yesterday,
		StartTime:         "13:00",
		EndTime:           "15:00",
		Observations:      "Quality check batch 1",
	})
	log.Printf("Created time entry: %s, QC 13:00-15:00", yesterday)

	database.Create(&models.TimeEntry{
		UserID:            workerUser.ID,
		ProductionOrderID: orderSteelBracket.ID,
		ServiceID:         svcAssembly.ID,
		Day:               today,
		StartTime:         "08:00",
		EndTime:           "11:30",
		Observations:      "Continue assembly",
	})
	log.Printf("Created time entry: %s, Assembly 08:00-11:30", today)

	// ========================================
	// Summary
	// ========================================
	var userCount, deviceCount, signalCount, valueCount int64
	var productCount, rawMaterialCount, bomCount, customerCount int64
	var orderCount, serviceCount, timeEntryCount int64
	database.Model(&models.User{}).Count(&userCount)
	database.Model(&models.Device{}).Count(&deviceCount)
	database.Model(&models.Signal{}).Count(&signalCount)
	database.Model(&models.SignalValue{}).Count(&valueCount)
	database.Model(&models.Product{}).Count(&productCount)
	database.Model(&models.RawMaterial{}).Count(&rawMaterialCount)
	database.Model(&models.BillOfMaterials{}).Count(&bomCount)
	database.Model(&models.Customer{}).Count(&customerCount)
	database.Model(&models.ProductionOrder{}).Count(&orderCount)
	database.Model(&models.Service{}).Count(&serviceCount)
	database.Model(&models.TimeEntry{}).Count(&timeEntryCount)

	log.Println("\n" + strings.Repeat("=", 50))
	log.Println("Database seeding completed successfully!")
	log.Println(strings.Repeat("=", 50))
	log.Printf("Users:             %d", userCount)
	log.Printf("Devices:           %d", deviceCount)
	log.Printf("Signals:           %d", signalCount)
	log.Printf("Signal Values:     %d", valueCount)
	log.Printf("Products:          %d", productCount)
	log.Printf("Raw Materials:     %d", rawMaterialCount)
	log.Printf("BOM Entries:       %d", bomCount)
	log.Printf("Customers:         %d", customerCount)
	log.Printf("Production Orders: %d", orderCount)
	log.Printf("Services:          %d", serviceCount)
	log.Printf("Time Entries:      %d", timeEntryCount)
	log.Println("\nUser Credentials:")
	log.Printf("  Admin: admin@test.com / admin123")
	log.Printf("  Worker: worker@test.com / worker123")
	log.Println("\nDevice Auth Tokens:")
	for _, device := range devices {
		log.Printf("  %s: %s", device.Name, device.AuthToken)
	}
	log.Println(strings.Repeat("=", 50))
}
