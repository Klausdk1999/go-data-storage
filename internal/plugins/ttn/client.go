package ttn

import (
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var client mqtt.Client

func connectMQTT(cfg Config) error {
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
		log.Println("[TTN Plugin] Connected to TTN broker")

		token := c.SubscribeMultiple(map[string]byte{
			uplinkTopic: 1,
			joinTopic:   1,
		}, func(client mqtt.Client, msg mqtt.Message) {
			handleMessage(msg.Topic(), msg.Payload())
		})

		if token.Wait() && token.Error() != nil {
			log.Printf("[TTN Plugin] Subscribe error: %v", token.Error())
		} else {
			log.Printf("[TTN Plugin] Subscribed to: %s", uplinkTopic)
			log.Printf("[TTN Plugin] Subscribed to: %s", joinTopic)
		}
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Printf("[TTN Plugin] Connection lost: %v", err)
	}

	client = mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return nil
}

func disconnectMQTT() {
	if client != nil && client.IsConnected() {
		client.Disconnect(250)
		log.Println("[TTN Plugin] MQTT disconnected")
	}
}
