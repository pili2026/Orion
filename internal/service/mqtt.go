// Package service is responsible for initializing and managing the MQTT client connection to the broker.
package service

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// createTLSConfig builds a TLS config using the CA cert at the given path.
// ServerName is read from the MQTT_BROKER env var so it stays consistent
// with the broker address and never needs to be hardcoded.
func createTLSConfig(caCertPath string) (*tls.Config, error) {
	ca, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate at %q: %w", caCertPath, err)
	}

	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("failed to parse CA certificate at %q: not valid PEM", caCertPath)
	}

	return &tls.Config{
		RootCAs: certpool,
		// Use the broker hostname from env so this never needs to be hardcoded.
		ServerName: os.Getenv("MQTT_BROKER"),
	}, nil
}

// InitMQTT initialises and connects the MQTT client.
// It returns an error instead of calling log.Fatalf so the caller
// (main.go) owns the shutdown decision and deferred cleanups still run.
func InitMQTT() (mqtt.Client, error) {
	broker := os.Getenv("MQTT_BROKER")
	port := os.Getenv("MQTT_PORT")
	if broker == "" || port == "" {
		return nil, fmt.Errorf("MQTT_BROKER and MQTT_PORT must be set")
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("ssl://%s:%s", broker, port))
	opts.SetClientID("orion-server")
	opts.SetUsername(os.Getenv("MQTT_USERNAME"))
	opts.SetPassword(os.Getenv("MQTT_PASSWORD"))

	// Enable automatic reconnection. The OnConnect handler (set below) will
	// re-subscribe on every successful connection, including reconnects.
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(false)

	if caCertPath := os.Getenv("MQTT_CA_CERT"); caCertPath != "" {
		tlsCfg, err := createTLSConfig(caCertPath)
		if err != nil {
			return nil, err
		}
		opts.SetTLSConfig(tlsCfg)
	}

	// NOTE: OnConnectHandler is intentionally left nil here.
	// main.go will call opts.SetOnConnectHandler after creating the Handler,
	// so the handler has access to h.SetupMQTTSubscribers().
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		// slog is used project-wide; import it if you add structured logging here.
		fmt.Printf("MQTT connection lost: %v\n", err)
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return client, nil
}
