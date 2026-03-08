package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`expected body["status"] == "ok", got %q`, body["status"])
	}
}

func TestUnknownRouteReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
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

func TestSetupMQTTSubscribers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, mqttMock, _, _, _, _ := newTestHandler()
	h.SetupMQTTSubscribers()

	expected := map[string]bool{
		"talos/+/telemetry": false,
		"talos/+/status":    false,
		"talos/+/event":     false,
		"talos/+/response":  false,
	}

	for _, topic := range mqttMock.SubscribedTopics {
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
