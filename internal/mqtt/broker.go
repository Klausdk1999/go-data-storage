package mqtt

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"data-storage/internal/db"
	"data-storage/internal/handlers"
	"data-storage/internal/models"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// BrokerConfig holds configuration for the embedded MQTT broker.
type BrokerConfig struct {
	Enabled    bool
	Port       string
	TLSEnabled bool
	TLSCert    string
	TLSKey     string
}

// LoadBrokerConfigFromEnv loads broker configuration from environment variables.
func LoadBrokerConfigFromEnv() BrokerConfig {
	enabled := os.Getenv("MQTT_BROKER_ENABLED") == "true"
	port := os.Getenv("MQTT_BROKER_PORT")
	if port == "" {
		port = "1883"
	}

	return BrokerConfig{
		Enabled:    enabled,
		Port:       port,
		TLSEnabled: os.Getenv("MQTT_BROKER_TLS_ENABLED") == "true",
		TLSCert:    os.Getenv("MQTT_BROKER_TLS_CERT"),
		TLSKey:     os.Getenv("MQTT_BROKER_TLS_KEY"),
	}
}

// ---------------------------------------------------------------------------
// Device Auth Hook — authenticates devices via username (device name) +
// password (auth_token) and enforces topic ACL.
// ---------------------------------------------------------------------------

// DeviceAuthHook validates MQTT connections against the device database.
type DeviceAuthHook struct {
	mochi.HookBase
}

func (h *DeviceAuthHook) ID() string {
	return "device-auth"
}

func (h *DeviceAuthHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mochi.OnConnectAuthenticate,
		mochi.OnACLCheck,
	}, []byte{b})
}

// OnConnectAuthenticate validates the username (device name) and password
// (auth_token) against the database.
func (h *DeviceAuthHook) OnConnectAuthenticate(cl *mochi.Client, pk packets.Packet) bool {
	username := string(pk.Connect.Username)
	password := string(pk.Connect.Password)

	if username == "" || password == "" {
		log.Printf("[MQTT Broker] Auth rejected: empty username or password from %s", cl.Net.Remote)
		return false
	}

	database := db.GetDB()
	var device models.Device
	result := database.Where("name = ? AND auth_token = ? AND is_active = ?", username, password, true).First(&device)
	if result.Error != nil {
		log.Printf("[MQTT Broker] Auth rejected: invalid credentials for device %q from %s", username, cl.Net.Remote)
		return false
	}

	log.Printf("[MQTT Broker] Auth accepted: device %q (ID=%d) from %s", device.Name, device.ID, cl.Net.Remote)
	return true
}

// OnACLCheck enforces that devices can only publish to devices/{their-name}/data.
func (h *DeviceAuthHook) OnACLCheck(cl *mochi.Client, topic string, write bool) bool {
	// Allow all reads (subscribes)
	if !write {
		return true
	}

	// For writes (publishes), the device can only publish to its own data topic.
	deviceName := string(cl.Properties.Username)
	allowedTopic := fmt.Sprintf("devices/%s/data", deviceName)

	if topic != allowedTopic {
		log.Printf("[MQTT Broker] ACL denied: device %q tried to publish to %q (allowed: %q)", deviceName, topic, allowedTopic)
		return false
	}

	return true
}

// ---------------------------------------------------------------------------
// Message Processing Hook — processes published messages and stores data.
// ---------------------------------------------------------------------------

// MessageProcessingHook intercepts published messages and stores device data.
type MessageProcessingHook struct {
	mochi.HookBase
}

func (h *MessageProcessingHook) ID() string {
	return "message-processing"
}

func (h *MessageProcessingHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mochi.OnPublished,
	}, []byte{b})
}

// OnPublished is called after a message has been published. It parses the
// topic (devices/+/data), extracts the device name, unmarshals the JSON
// payload into GenericDeviceDataRaw, and calls ProcessGenericDeviceData.
func (h *MessageProcessingHook) OnPublished(cl *mochi.Client, pk packets.Packet) {
	// Only process messages on the devices/+/data topic pattern
	parts := strings.Split(pk.TopicName, "/")
	if len(parts) != 3 || parts[0] != "devices" || parts[2] != "data" {
		return
	}

	deviceName := parts[1]

	var data handlers.GenericDeviceDataRaw
	if err := json.Unmarshal(pk.Payload, &data); err != nil {
		log.Printf("[MQTT Broker] Error parsing payload from device %q: %v", deviceName, err)
		return
	}

	// Ensure device_id is set from the topic if not present in payload
	if data.DeviceID == "" {
		data.DeviceID = deviceName
	}

	if err := handlers.ProcessGenericDeviceData(data, ""); err != nil {
		log.Printf("[MQTT Broker] Error processing data from device %q: %v", deviceName, err)
		return
	}

	log.Printf("[MQTT Broker] Processed data from device %q on topic %q", deviceName, pk.TopicName)
}

// ---------------------------------------------------------------------------
// StartBroker creates and starts the embedded MQTT broker.
// ---------------------------------------------------------------------------

// StartBroker creates a new mochi-mqtt server, attaches the auth and message
// processing hooks, adds a TCP listener, and starts serving in a goroutine.
// Returns nil server if the broker is not enabled.
func StartBroker(cfg BrokerConfig) (*mochi.Server, error) {
	if !cfg.Enabled {
		log.Println("[MQTT Broker] Embedded MQTT broker is disabled")
		return nil, nil
	}

	server := mochi.New(nil)

	// Add device authentication hook
	if err := server.AddHook(new(DeviceAuthHook), nil); err != nil {
		return nil, fmt.Errorf("failed to add device auth hook: %w", err)
	}

	// Add message processing hook
	if err := server.AddHook(new(MessageProcessingHook), nil); err != nil {
		return nil, fmt.Errorf("failed to add message processing hook: %w", err)
	}

	// Configure TCP listener
	address := ":" + cfg.Port
	tcpConfig := listeners.Config{
		ID:      "broker-tcp",
		Address: address,
	}

	// Configure TLS if enabled
	if cfg.TLSEnabled && cfg.TLSCert != "" && cfg.TLSKey != "" {
		tlsConfig, err := loadTLSConfig(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
		tcpConfig.TLSConfig = tlsConfig
	}

	tcp := listeners.NewTCP(tcpConfig)
	if err := server.AddListener(tcp); err != nil {
		return nil, fmt.Errorf("failed to add TCP listener: %w", err)
	}

	// Start the broker in a goroutine
	go func() {
		if err := server.Serve(); err != nil {
			log.Printf("[MQTT Broker] Server error: %v", err)
		}
	}()

	log.Printf("[MQTT Broker] Started on %s (TLS=%v)", address, cfg.TLSEnabled)
	return server, nil
}

// loadTLSConfig creates a tls.Config from certificate and key file paths.
func loadTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS key pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}
