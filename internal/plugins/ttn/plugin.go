package ttn

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// Config holds TTN plugin configuration.
type Config struct {
	Enabled  bool
	Broker   string
	Username string
	Password string
}

// LoadConfig reads TTN configuration from environment variables.
func LoadConfig() Config {
	return Config{
		Enabled:  os.Getenv("TTN_ENABLED") == "true",
		Broker:   os.Getenv("TTN_MQTT_BROKER"),
		Username: os.Getenv("TTN_USERNAME"),
		Password: os.Getenv("TTN_PASSWORD"),
	}
}

// Register initializes the TTN plugin: connects the MQTT client and registers
// HTTP routes. Returns a cleanup function. No-ops if TTN_ENABLED != "true".
func Register(router *mux.Router, authMiddleware func(http.HandlerFunc) http.HandlerFunc) func() {
	cfg := LoadConfig()

	if !cfg.Enabled {
		log.Println("[TTN Plugin] Disabled (set TTN_ENABLED=true to enable)")
		return func() {}
	}

	log.Println("[TTN Plugin] Initializing...")

	// Connect MQTT client
	if cfg.Broker != "" && cfg.Username != "" && cfg.Password != "" {
		if err := connectMQTT(cfg); err != nil {
			log.Printf("[TTN Plugin] Warning: Failed to connect to MQTT broker: %v", err)
			log.Println("[TTN Plugin] Continuing without MQTT - TTN data collection will not work")
		}
	} else {
		log.Println("[TTN Plugin] MQTT not configured - skipping TTN MQTT connection")
	}

	// Register HTTP routes
	registerRoutes(router, authMiddleware)

	log.Println("[TTN Plugin] Ready")

	return func() {
		disconnectMQTT()
		log.Println("[TTN Plugin] Shut down")
	}
}
