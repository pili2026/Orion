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
//
// ServerName precedence:
//  1. MQTT_TLS_SERVER_NAME  — explicit override (use when broker hostname differs from cert CN/SAN)
//  2. MQTT_BROKER           — fallback (works when both are the same, e.g. production)
//
// Local dev example:
//
//	MQTT_BROKER=localhost
//	MQTT_TLS_SERVER_NAME=mqtt.eversource-ai.com
//
// Production example (same value, no override needed):
//
//	MQTT_BROKER=mqtt.eversource-ai.com
func createTLSConfig(caCertPath string) (*tls.Config, error) {
	ca, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate at %q: %w", caCertPath, err)
	}

	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("failed to parse CA certificate at %q: not valid PEM", caCertPath)
	}

	serverName := os.Getenv("MQTT_TLS_SERVER_NAME")
	if serverName == "" {
		serverName = os.Getenv("MQTT_BROKER")
	}

	return &tls.Config{
		RootCAs:    certpool,
		ServerName: serverName,
	}, nil
}

// InitMQTT initialises and connects the MQTT client.
// Returns an error instead of calling log.Fatalf so the caller (main.go)
// owns the shutdown decision and deferred cleanups still run.
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

	opts.SetAutoReconnect(true)
	opts.SetCleanSession(false)

	if caCertPath := os.Getenv("MQTT_CA_CERT"); caCertPath != "" {
		tlsCfg, err := createTLSConfig(caCertPath)
		if err != nil {
			return nil, err
		}
		opts.SetTLSConfig(tlsCfg)
	}

	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		fmt.Printf("MQTT connection lost: %v\n", err)
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return client, nil
}
