package handler

import (
	"log/slog"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// SetupMQTTSubscribers registers all MQTT subscriptions.
// It is called both at startup and from the OnConnect handler so that
// subscriptions are automatically restored after a broker reconnection.
// (Paho's auto-reconnect restores the TCP connection but NOT subscriptions
// when CleanSession is false and the broker has no stored session for this client.)
func (h *Handler) SetupMQTTSubscribers() {
	subscriptions := map[string]mqtt.MessageHandler{
		// Correct wildcard: talos/{edge_id}/telemetry
		// "+" matches exactly one level (the edge_id).
		// The previous "talos/telemetry/#" would never match any Talos message.
		"talos/+/telemetry": h.handleTelemetry,
		"talos/+/status":    h.handleStatus,
		"talos/+/event":     h.handleEvent,
		"talos/+/response":  h.handleResponse,
	}

	for topic, handler := range subscriptions {
		token := h.MQTT.Subscribe(topic, 1, handler)
		token.Wait()
		if token.Error() != nil {
			slog.Error("Failed to subscribe to MQTT topic",
				slog.String("topic", topic),
				slog.Any("error", token.Error()),
			)
			continue
		}
		slog.Info("Subscribed to MQTT topic", slog.String("topic", topic))
	}
}

// handleTelemetry processes incoming telemetry payloads from Edge devices.
// TODO: parse payload, validate schema, then write to TimescaleDB via pgxpool.
func (h *Handler) handleTelemetry(_ mqtt.Client, msg mqtt.Message) {
	slog.Info("MQTT telemetry received",
		slog.String("topic", msg.Topic()),
		slog.String("payload", string(msg.Payload())),
	)
}

// handleStatus processes status updates (online/offline heartbeats).
// TODO: update gateways.last_seen_at and network_status via GORM.
func (h *Handler) handleStatus(_ mqtt.Client, msg mqtt.Message) {
	slog.Info("MQTT status received",
		slog.String("topic", msg.Topic()),
		slog.String("payload", string(msg.Payload())),
	)
}

// handleEvent processes event notifications from Edge devices.
func (h *Handler) handleEvent(_ mqtt.Client, msg mqtt.Message) {
	slog.Info("MQTT event received",
		slog.String("topic", msg.Topic()),
		slog.String("payload", string(msg.Payload())),
	)
}

// handleResponse processes command-response acknowledgements from Edge devices.
func (h *Handler) handleResponse(_ mqtt.Client, msg mqtt.Message) {
	slog.Info("MQTT response received",
		slog.String("topic", msg.Topic()),
		slog.String("payload", string(msg.Payload())),
	)
}
