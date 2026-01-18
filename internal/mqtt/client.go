package mqtt

import (
	"fmt"
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var client mqtt.Client

type Config struct {
	Broker   string
	Username string
	Password string
}

func LoadConfigFromEnv() Config {
	return Config{
		Broker:   os.Getenv("TTN_MQTT_BROKER"),
		Username: os.Getenv("TTN_USERNAME"),
		Password: os.Getenv("TTN_PASSWORD"),
	}
}

func ConnectMQTT(cfg Config, messageHandler func(string, []byte)) error {
	if cfg.Broker == "" || cfg.Username == "" || cfg.Password == "" {
		return fmt.Errorf("MQTT configuration incomplete: broker, username, and password are required")
	}

	// TTN topic pattern: v3/{application_id}/devices/+/up
	uplinkTopic := fmt.Sprintf("v3/%s/devices/+/up", cfg.Username)
	joinTopic := fmt.Sprintf("v3/%s/devices/+/join", cfg.Username)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)
	opts.SetClientID(fmt.Sprintf("go-data-storage-%d", time.Now().Unix()))
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetConnectTimeout(30 * time.Second)

	opts.OnConnect = func(c mqtt.Client) {
		log.Println("[MQTT] Connected to TTN broker")
		
		token := c.SubscribeMultiple(map[string]byte{
			uplinkTopic: 1,
			joinTopic:   1,
		}, func(client mqtt.Client, msg mqtt.Message) {
			messageHandler(msg.Topic(), msg.Payload())
		})

		if token.Wait() && token.Error() != nil {
			log.Printf("[MQTT] Subscribe error: %v", token.Error())
		} else {
			log.Printf("[MQTT] Subscribed to: %s", uplinkTopic)
			log.Printf("[MQTT] Subscribed to: %s", joinTopic)
		}
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Printf("[MQTT] Connection lost: %v", err)
	}

	client = mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return nil
}

func DisconnectMQTT() {
	if client != nil && client.IsConnected() {
		client.Disconnect(250)
		log.Println("[MQTT] Disconnected")
	}
}

func GetClient() mqtt.Client {
	return client
}
