// Package service is responsible for initializing and managing the MQTT client connection to the broker.
package service

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func createTLSConfig(caCertPath string) *tls.Config {
	certpool := x509.NewCertPool()
	ca, err := os.ReadFile(caCertPath)
	if err != nil {
		log.Fatalf("Failed to read CA certificate: %v", err)
	}
	certpool.AppendCertsFromPEM(ca)

	return &tls.Config{
		RootCAs:    certpool,
		ServerName: "mqtt.eversource-ai.com", // Replace with your MQTT broker's hostname if different
	}
}

func InitMQTT() mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("ssl://%s:%s", os.Getenv("MQTT_BROKER"), os.Getenv("MQTT_PORT")))
	opts.SetClientID("orion-server")
	opts.SetUsername(os.Getenv("MQTT_USERNAME"))
	opts.SetPassword(os.Getenv("MQTT_PASSWORD"))

	if caCertPath := os.Getenv("MQTT_CA_CERT"); caCertPath != "" {
		opts.SetTLSConfig(createTLSConfig(caCertPath))
	}

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		fmt.Println("MQTT connected!")
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	})

	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to connect to MQTT: %v", token.Error())
	}
	return mqttClient
}
