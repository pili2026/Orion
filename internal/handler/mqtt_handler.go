package handler

import (
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func (h *Handler) SetupMQTTSubscribers() {
	topic := "talos/telemetry/#"
	qos := byte(1)

	token := h.MQTT.Subscribe(topic, qos, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("📥 [MQTT Receive] Source topic: %s", msg.Topic())
		log.Printf("📦 [MQTT Payload] Payload: %s\n", string(msg.Payload()))
		// Here you can add code to process the incoming telemetry data, e.g., save to DB
	})

	token.Wait()
	if token.Error() != nil {
		log.Printf("Failed to subscribe to topic %s: %v", topic, token.Error())
	}

	log.Printf("Subscribed to MQTT topic: %s", topic)
}
