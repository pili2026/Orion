package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
)

// ── MQTT mock helpers ────────────────────────────────────────────────────────

// mockToken satisfies mqtt.Token without doing any real network work.
// We only need Wait() and Error() for the Subscribe call in SetupMQTTSubscribers.
type mockToken struct{ mqtt.Token }

func (m *mockToken) Wait() bool   { return true }
func (m *mockToken) Error() error { return nil }

// mockMQTTClient records Subscribe/Publish calls so tests can assert on them
// without touching a real broker.
type mockMQTTClient struct {
	mqtt.Client // embed to satisfy the full interface with zero-value no-ops

	SubscribedTopics []string
	PublishedTopic   string
	PublishedPayload interface{}
}

func (m *mockMQTTClient) Subscribe(topic string, _ byte, _ mqtt.MessageHandler) mqtt.Token {
	m.SubscribedTopics = append(m.SubscribedTopics, topic)
	return &mockToken{}
}

func (m *mockMQTTClient) Publish(topic string, _ byte, _ bool, payload interface{}) mqtt.Token {
	m.PublishedTopic = topic
	m.PublishedPayload = payload
	return &mockToken{}
}

func (m *mockMQTTClient) AddRoute(_ string, _ mqtt.MessageHandler) {}

// ── Test helpers ─────────────────────────────────────────────────────────────

// newTestHandler builds a Handler with nil DB (acceptable for routes that
// don't touch the database) and a fresh mockMQTTClient.
func newTestHandler() (*Handler, *mockMQTTClient) {
	mock := &mockMQTTClient{}
	// nil DBManager is fine for handler-level tests that don't hit the DB.
	h := NewHandler(nil, mock)
	return h, mock
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _ := newTestHandler()
	r := h.SetupRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Status
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Body: {"status":"ok"}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`expected body["status"] == "ok", got %q`, body["status"])
	}
}

// TestUnknownRouteReturns404 ensures the router does not accidentally expose
// old debug endpoints (e.g. the removed /publish route).
func TestUnknownRouteReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _ := newTestHandler()
	r := h.SetupRouter()

	for _, path := range []string{"/publish", "/debug", "/api/v99/unknown"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("path %q: expected 404, got %d", path, w.Code)
		}
	}
}

// TestSetupMQTTSubscribers verifies that all four expected uplink topics
// are subscribed when SetupMQTTSubscribers is called.
func TestSetupMQTTSubscribers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, mock := newTestHandler()
	h.SetupMQTTSubscribers()

	expected := map[string]bool{
		"talos/+/telemetry": false,
		"talos/+/status":    false,
		"talos/+/event":     false,
		"talos/+/response":  false,
	}

	for _, topic := range mock.SubscribedTopics {
		if _, ok := expected[topic]; ok {
			expected[topic] = true
		}
	}

	for topic, found := range expected {
		if !found {
			t.Errorf("expected subscription to %q was not registered", topic)
		}
	}
}
