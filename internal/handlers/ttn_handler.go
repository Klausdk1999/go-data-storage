package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"data-storage/internal/db"
	"data-storage/internal/models"
)

// TTNUplink represents a formatted uplink for TTN API
type TTNUplink struct {
	ID             uint      `json:"id"`
	DeviceID       string    `json:"device_id"`
	DevEUI         string    `json:"dev_eui"`
	ReceivedAt     time.Time `json:"received_at"`
	PayloadFormat  string    `json:"payload_format,omitempty"`

	// Distance measurements
	DistanceMm          int16   `json:"distance_mm"`
	DistanceCm          float64 `json:"distance_cm"`
	UltrasonicDistMm    *int16  `json:"ultrasonic_distance_mm,omitempty"`
	UltrasonicDistCm    *float64 `json:"ultrasonic_distance_cm,omitempty"`

	// Temperature & humidity
	Temperature    int8   `json:"temperature"`
	AmbientTemp    *int8  `json:"ambient_temp,omitempty"`
	SensorTemp     *int8  `json:"sensor_temp,omitempty"`
	Humidity       *uint8 `json:"humidity,omitempty"`

	// Battery
	BatteryPercent uint8   `json:"battery_percent"`
	BatteryMv      *uint16 `json:"battery_mv,omitempty"`

	// Sensor info
	SignalStrength int16  `json:"signal_strength"`
	ReadingCount   uint8  `json:"reading_count"`
	SensorFlags    *uint8 `json:"sensor_flags,omitempty"`

	// LoRa metadata
	RSSI      int     `json:"rssi"`
	SNR       float64 `json:"snr"`
	GatewayID string  `json:"gateway_id"`
	FCnt      int     `json:"f_cnt"`
}

// TTNDevice represents a TTN device summary
type TTNDevice struct {
	DeviceID    string    `json:"device_id"`
	DevEUI      string    `json:"dev_eui"`
	LastSeen    time.Time `json:"last_seen"`
	UplinkCount int64     `json:"uplink_count"`
}

// TTNStats represents statistics
type TTNStats struct {
	TotalUplinks  int64     `json:"total_uplinks"`
	UniqueDevices int64     `json:"unique_devices"`
	FirstUplink   time.Time `json:"first_uplink,omitempty"`
	LastUplink    time.Time `json:"last_uplink,omitempty"`
}

// TTNUplinksHandler handles GET /ttn/uplinks
func TTNUplinksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	database := db.GetDB()

	// Build query for distance signal values (TTN devices use "distance_mm" signal name)
	var signalValues []models.SignalValue
	query := database.
		Joins("JOIN signals ON signal_values.signal_id = signals.id").
		Joins("JOIN devices ON signals.device_id = devices.id").
		Where("signals.name = ?", "distance_mm").
		Preload("Signal").
		Preload("Signal.Device")

	// Filter by device_id (device name in our system matches TTN device_id)
	if deviceID := r.URL.Query().Get("device_id"); deviceID != "" {
		query = query.Where("devices.name = ?", deviceID)
	}

	// Date range filters
	if startDate := r.URL.Query().Get("start_date"); startDate != "" {
		if parsed, err := time.Parse(time.RFC3339, startDate); err == nil {
			query = query.Where("signal_values.timestamp >= ?", parsed)
		}
	}
	if endDate := r.URL.Query().Get("end_date"); endDate != "" {
		if parsed, err := time.Parse(time.RFC3339, endDate); err == nil {
			query = query.Where("signal_values.timestamp <= ?", parsed)
		}
	}

	// Limit
	limit := 1000
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 10000 {
				limit = 10000 // Max limit
			}
		}
	}

	result := query.Order("signal_values.timestamp DESC").Limit(limit).Find(&signalValues)
	if result.Error != nil {
		log.Printf("Error fetching TTN uplinks: %v", result.Error)
		http.Error(w, "Error fetching uplinks", http.StatusInternalServerError)
		return
	}

	// Convert to TTNUplink format
	uplinks := make([]TTNUplink, 0, len(signalValues))
	for _, sv := range signalValues {
		// Get related values from same timestamp (battery, temp, etc.)
		var relatedValues []models.SignalValue
		database.Where("timestamp = ? AND signal_id IN (SELECT id FROM signals WHERE device_id = ?)",
			sv.Timestamp, sv.Signal.DeviceID).Preload("Signal").Find(&relatedValues)

		uplink := TTNUplink{
			ID:         sv.ID,
			DeviceID:   sv.Signal.Device.Name,
			ReceivedAt: sv.Timestamp,
			DistanceMm: int16(*sv.Value),
			DistanceCm: *sv.Value / 10.0,
		}

		// Extract metadata and related values
		if sv.Metadata != nil {
			if gwID, ok := sv.Metadata["gateway_id"].(string); ok {
				uplink.GatewayID = gwID
			}
			if rssi, ok := sv.Metadata["rssi"].(float64); ok {
				uplink.RSSI = int(rssi)
			}
			if snr, ok := sv.Metadata["snr"].(float64); ok {
				uplink.SNR = snr
			}
			if fCnt, ok := sv.Metadata["f_cnt"].(float64); ok {
				uplink.FCnt = int(fCnt)
			}
			if format, ok := sv.Metadata["payload_format"].(string); ok {
				uplink.PayloadFormat = format
			}
		}

		// Find all related signal values
		for _, rv := range relatedValues {
			if rv.Value == nil {
				continue
			}
			switch rv.Signal.Name {
			case "battery_percent":
				uplink.BatteryPercent = uint8(*rv.Value)
			case "temperature":
				uplink.Temperature = int8(*rv.Value)
			case "signal_strength":
				uplink.SignalStrength = int16(*rv.Value)
			case "reading_count":
				uplink.ReadingCount = uint8(*rv.Value)
			case "humidity":
				h := uint8(*rv.Value)
				uplink.Humidity = &h
			case "ambient_temp":
				t := int8(*rv.Value)
				uplink.AmbientTemp = &t
			case "sensor_temp":
				t := int8(*rv.Value)
				uplink.SensorTemp = &t
			case "ultrasonic_distance_mm":
				d := int16(*rv.Value)
				uplink.UltrasonicDistMm = &d
				cm := *rv.Value / 10.0
				uplink.UltrasonicDistCm = &cm
			case "battery_mv":
				mv := uint16(*rv.Value)
				uplink.BatteryMv = &mv
			case "sensor_flags":
				f := uint8(*rv.Value)
				uplink.SensorFlags = &f
			}
		}

		// Get DevEUI from device auth_token (we store it there)
		uplink.DevEUI = sv.Signal.Device.AuthToken

		uplinks = append(uplinks, uplink)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(uplinks)
}

// TTNDevicesHandler handles GET /ttn/devices
func TTNDevicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	database := db.GetDB()

	// Find all devices that have "distance_mm" signals (TTN devices)
	var devices []struct {
		DeviceID   string
		DevEUI     string
		LastSeen   time.Time
		Count      int64
	}

	result := database.Table("devices").
		Select("devices.name as device_id, devices.auth_token as dev_eui, MAX(signal_values.timestamp) as last_seen, COUNT(*) as count").
		Joins("JOIN signals ON signals.device_id = devices.id").
		Joins("JOIN signal_values ON signal_values.signal_id = signals.id").
		Where("signals.name = ?", "distance_mm").
		Group("devices.id, devices.name, devices.auth_token").
		Find(&devices)

	if result.Error != nil {
		log.Printf("Error fetching TTN devices: %v", result.Error)
		http.Error(w, "Error fetching devices", http.StatusInternalServerError)
		return
	}

	ttnDevices := make([]TTNDevice, len(devices))
	for i, d := range devices {
		ttnDevices[i] = TTNDevice{
			DeviceID:    d.DeviceID,
			DevEUI:      d.DevEUI,
			LastSeen:    d.LastSeen,
			UplinkCount: d.Count,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ttnDevices)
}

// TTNStatsHandler handles GET /ttn/stats
func TTNStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	database := db.GetDB()

	stats := TTNStats{}

	// Count total uplinks (count distance_mm signal values)
	var count int64
	database.Table("signal_values").
		Joins("JOIN signals ON signal_values.signal_id = signals.id").
		Where("signals.name = ?", "distance_mm").
		Count(&count)
	stats.TotalUplinks = count

	// Count unique devices
	database.Table("devices").
		Joins("JOIN signals ON signals.device_id = devices.id").
		Where("signals.name = ?", "distance_mm").
		Distinct("devices.id").
		Count(&count)
	stats.UniqueDevices = count

	// Get first and last uplink
	var firstValue, lastValue models.SignalValue
	database.Table("signal_values").
		Joins("JOIN signals ON signal_values.signal_id = signals.id").
		Where("signals.name = ?", "distance_mm").
		Order("signal_values.timestamp ASC").
		First(&firstValue)
	
	database.Table("signal_values").
		Joins("JOIN signals ON signal_values.signal_id = signals.id").
		Where("signals.name = ?", "distance_mm").
		Order("signal_values.timestamp DESC").
		First(&lastValue)

	if firstValue.ID != 0 {
		stats.FirstUplink = firstValue.Timestamp
	}
	if lastValue.ID != 0 {
		stats.LastUplink = lastValue.Timestamp
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
