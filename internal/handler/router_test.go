package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
)

// ── MQTT mock ────────────────────────────────────────────────────────────────

type mockToken struct{ mqtt.Token }

func (m *mockToken) Wait() bool   { return true }
func (m *mockToken) Error() error { return nil }

type mockMQTTClient struct {
	mqtt.Client
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

// ── GatewayService mock ──────────────────────────────────────────────────────

// mockGatewayService implements the GatewayService interface.
// Each method can be overridden per-test by setting the corresponding func field.
type mockGatewayService struct {
	RegisterFn func(ctx context.Context, req dto.CreateGatewayRequest) (*dto.RegisterGatewayResponse, error)
	ListFn     func(ctx context.Context) ([]dto.GatewayResponse, error)
	GetByIDFn  func(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error)
	UpdateFn   func(ctx context.Context, id uuid.UUID, req dto.UpdateGatewayRequest) (*dto.GatewayResponse, error)
	DeleteFn   func(ctx context.Context, id uuid.UUID) error
}

func (m *mockGatewayService) Register(ctx context.Context, req dto.CreateGatewayRequest) (*dto.RegisterGatewayResponse, error) {
	return m.RegisterFn(ctx, req)
}
func (m *mockGatewayService) List(ctx context.Context) ([]dto.GatewayResponse, error) {
	return m.ListFn(ctx)
}
func (m *mockGatewayService) GetByID(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error) {
	return m.GetByIDFn(ctx, id)
}
func (m *mockGatewayService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateGatewayRequest) (*dto.GatewayResponse, error) {
	return m.UpdateFn(ctx, id, req)
}
func (m *mockGatewayService) Delete(ctx context.Context, id uuid.UUID) error {
	return m.DeleteFn(ctx, id)
}

// ── Test helper ──────────────────────────────────────────────────────────────

func newTestHandler() (*Handler, *mockMQTTClient, *mockGatewayService) {
	mqttMock := &mockMQTTClient{}
	svcMock := &mockGatewayService{}
	// nil DBManager is acceptable for handler-level tests that don't hit the DB.
	h := NewHandler(nil, mqttMock, svcMock)
	return h, mqttMock, svcMock
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _ := newTestHandler()
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

	h, _, _ := newTestHandler()
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

	h, mqttMock, _ := newTestHandler()
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

func TestListGateways_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, svc := newTestHandler()

	svc.ListFn = func(_ context.Context) ([]dto.GatewayResponse, error) {
		return []dto.GatewayResponse{
			{ID: "abc-123", DisplayName: "Test GW", Status: "offline"},
		}, nil
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body []dto.GatewayResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if len(body) != 1 || body[0].ID != "abc-123" {
		t.Errorf("unexpected response body: %+v", body)
	}
}

func TestGetGateway_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid UUID, got %d", w.Code)
	}
}
