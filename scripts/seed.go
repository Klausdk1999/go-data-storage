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

	// Clear existing data (optional - comment out if you want to keep existing data)
	log.Println("Clearing existing data...")
	database.Exec("TRUNCATE TABLE signal_values CASCADE")
	database.Exec("TRUNCATE TABLE signals CASCADE")
	database.Exec("TRUNCATE TABLE devices CASCADE")
	database.Exec("TRUNCATE TABLE users CASCADE")

	// 1. Create a test user
	user := models.User{
		Name:      "Test User",
		Email:     "test@example.com",
		Categoria: "Admin",
		Matricula: "12345",
		Rfid:      "RFID001",
		IsActive:  true,
	}
	err = user.SetPassword("password123")
	if err != nil {
		log.Fatalf("Failed to set password: %v", err)
	}

	result := database.Create(&user)
	if result.Error != nil {
		log.Fatalf("Failed to create user: %v", result.Error)
	}
	log.Printf("✓ Created user: %s (ID: %d, Email: %s, Password: password123)", user.Name, user.ID, user.Email)

	// 2. Create devices
	devices := []models.Device{
		{
			Name:        "Temperature Sensor 1",
			Description: "Main temperature sensor in living room",
			DeviceType:  "sensor",
			Location:    "Living Room",
			UserID:      &user.ID,
			IsActive:    true,
		},
		{
			Name:        "Humidity Sensor 1",
			Description: "Humidity sensor in kitchen",
			DeviceType:  "sensor",
			Location:    "Kitchen",
			UserID:      &user.ID,
			IsActive:    true,
		},
		{
			Name:        "Smart Light Switch",
			Description: "Smart switch controlling living room lights",
			DeviceType:  "actuator",
			Location:    "Living Room",
			UserID:      &user.ID,
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
		log.Printf("✓ Created device: %s (ID: %d, Token: %s)", devices[i].Name, devices[i].ID, devices[i].AuthToken)
	}

	// 3. Create signals for each device
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
	log.Printf("✓ Created signal: %s (ID: %d)", temperatureSignal.Name, temperatureSignal.ID)

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
	log.Printf("✓ Created signal: %s (ID: %d)", humiditySignal.Name, humiditySignal.ID)

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
	log.Printf("✓ Created signal: %s (ID: %d)", pressureSignal.Name, pressureSignal.ID)

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
	log.Printf("✓ Created signal: %s (ID: %d)", lightSwitchSignal.Name, lightSwitchSignal.ID)

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
	log.Printf("✓ Created signal: %s (ID: %d)", motionSensorSignal.Name, motionSensorSignal.ID)

	// 4. Create signal values (historical data)
	log.Println("\nCreating signal values...")

	// Temperature values (last 24 hours, every hour)
	rand.Seed(time.Now().UnixNano())
	baseTime := now.Add(-24 * time.Hour)
	for i := 0; i < 24; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		// Generate realistic temperature between 18-25°C with some variation
		tempValue := 20.0 + (rand.Float64() * 7.0) + (math.Sin(float64(i)/3.0) * 2.0)
		value := models.SignalValue{
			SignalID:  temperatureSignal.ID,
			UserID:    &user.ID,
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
			UserID:    &user.ID,
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
			UserID:    &user.ID,
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
			UserID:       &user.ID,
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
		motionDetected := rand.Float64() > 0.7 // 30% chance of motion
		value := models.SignalValue{
			SignalID:     motionSensorSignal.ID,
			UserID:       &user.ID,
			Timestamp:    timestamp,
			DigitalValue: &motionDetected,
			Metadata:     models.JSONB{"source": "sensor"},
		}
		database.Create(&value)
	}

	// Get counts
	var userCount, deviceCount, signalCount, valueCount int64
	database.Model(&models.User{}).Count(&userCount)
	database.Model(&models.Device{}).Count(&deviceCount)
	database.Model(&models.Signal{}).Count(&signalCount)
	database.Model(&models.SignalValue{}).Count(&valueCount)

	log.Println("\n" + strings.Repeat("=", 50))
	log.Println("✓ Database seeding completed successfully!")
	log.Println(strings.Repeat("=", 50))
	log.Printf("Users: %d", userCount)
	log.Printf("Devices: %d", deviceCount)
	log.Printf("Signals: %d", signalCount)
	log.Printf("Signal Values: %d", valueCount)
	log.Println("\nTest User Credentials:")
	log.Printf("  Email: %s", user.Email)
	log.Printf("  Password: password123")
	log.Println("\nDevice Auth Tokens:")
	for _, device := range devices {
		log.Printf("  %s: %s", device.Name, device.AuthToken)
	}
	log.Println(strings.Repeat("=", 50))
}

