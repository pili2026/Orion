// Package service is responsible for initializing and managing the MQTT client connection to the broker.
package service

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// createTLSConfig builds a TLS config using the CA cert at the given path.
//
// ServerName precedence:
//  1. MQTT_TLS_SERVER_NAME  — explicit override (use when broker hostname differs from cert CN/SAN)
//  2. MQTT_BROKER           — fallback (works when both are the same, e.g. production)
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
//
// onConnect is called every time the client successfully connects or reconnects
// to the broker — including the initial connection and any automatic reconnects.
// This ensures MQTT subscriptions are always restored after a broker restart or
// network interruption (Paho's AutoReconnect restores the TCP connection but
// does NOT restore subscriptions when CleanSession=false and the broker has no
// stored session for this client).
//
// Pass nil if no post-connect callback is needed.
func InitMQTT(onConnect func()) (mqtt.Client, error) {
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

	// OnConnect is triggered on every successful (re)connection.
	// Subscriptions must be re-registered here because Paho does not
	// restore them automatically after a reconnect.
	opts.SetOnConnectHandler(func(_ mqtt.Client) {
		slog.Info("MQTT connected")
		if onConnect != nil {
			onConnect()
		}
	})

	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		slog.Warn("MQTT connection lost", slog.Any("error", err))
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return client, nil
}
