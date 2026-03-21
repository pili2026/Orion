package handler

import (
	"context"
	"log/slog"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTIngestService is the interface the MQTT handler depends on.
// The concrete implementation is service.MQTTIngestService.
type MQTTIngestService interface {
	// ProcessTelemetry parses a raw MQTT payload and writes all readings to
	// TimescaleDB.  Returns a non-nil error only for unparseable payloads;
	// transient DB / resolution failures are handled internally via the DLQ.
	ProcessTelemetry(ctx context.Context, mqttUsername string, payload []byte) error
}

// SetupMQTTSubscribers registers all MQTT topic subscriptions.
// Called at startup and again on every broker reconnection (Paho's auto-reconnect
// restores the TCP connection but not subscriptions when CleanSession=false).
func (h *Handler) SetupMQTTSubscribers() {
	subscriptions := map[string]mqtt.MessageHandler{
		"talos/+/telemetry": h.handleTelemetry,
		"talos/+/status":    h.handleStatus,
		"talos/+/event":     h.handleEvent,
		"talos/+/response":  h.handleResponse,
	}

	for topic, fn := range subscriptions {
		token := h.MQTT.Subscribe(topic, 1, fn)
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

// handleTelemetry processes incoming telemetry payloads from Talos Edge devices.
//
// Topic format: talos/{mqtt_username}/telemetry
//
// The handler extracts the mqtt_username from the topic, then delegates all
// parsing and DB writes to MQTTIngestService in a dedicated goroutine so the
// Paho message dispatcher is never blocked.
func (h *Handler) handleTelemetry(_ mqtt.Client, msg mqtt.Message) {
	mqttUsername, ok := extractMQTTUsername(msg.Topic())
	if !ok {
		slog.Warn("handleTelemetry: malformed topic, ignoring",
			slog.String("topic", msg.Topic()),
		)
		return
	}

	// Capture payload bytes — msg is only valid for the duration of this
	// callback, so copy before spawning the goroutine.
	payload := make([]byte, len(msg.Payload()))
	copy(payload, msg.Payload())

	go func() {
		if err := h.IngestSvc.ProcessTelemetry(context.Background(), mqttUsername, payload); err != nil {
			slog.Warn("handleTelemetry: unparseable payload",
				slog.String("topic", msg.Topic()),
				slog.Any("error", err),
			)
		}
	}()
}

// handleStatus processes online/offline heartbeat messages.
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

// extractMQTTUsername parses the middle segment from a three-level topic.
//
//	"talos/my-edge-01/telemetry" → "my-edge-01", true
//	"bad/format"                  → "", false
func extractMQTTUsername(topic string) (string, bool) {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
