package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/pkg/apperr"
)

func TestListGateways_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, svc, _, _, _ := newTestHandler()
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
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestListGateways_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, svc, _, _, _ := newTestHandler()
	svc.ListFn = func(_ context.Context) ([]dto.GatewayResponse, error) {
		return nil, errors.New("db unavailable")
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetGateway_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, svc, _, _, _ := newTestHandler()
	svc.GetByIDFn = func(_ context.Context, got uuid.UUID) (*dto.GatewayResponse, error) {
		if got != id {
			t.Errorf("expected id %s, got %s", id, got)
		}
		return &dto.GatewayResponse{ID: id.String(), DisplayName: "GW-01", Status: "online"}, nil
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetGateway_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetGateway_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, svc, _, _, _ := newTestHandler()
	svc.GetByIDFn = func(_ context.Context, _ uuid.UUID) (*dto.GatewayResponse, error) {
		return nil, apperr.ErrNotFound
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestRegisterGateway_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, svc, _, _, _ := newTestHandler()
	svc.RegisterFn = func(_ context.Context, req dto.CreateGatewayRequest) (*dto.RegisterGatewayResponse, error) {
		return &dto.RegisterGatewayResponse{
			Gateway:      dto.GatewayResponse{ID: uuid.New().String(), DisplayName: req.DisplayName, Status: "offline"},
			MQTTPassword: "one-time-secret",
		}, nil
	}

	body, _ := json.Marshal(dto.CreateGatewayRequest{
		SerialNo:    "SN-001",
		Mac:         "AA:BB:CC:DD:EE:FF",
		Model:       "RPi4",
		DisplayName: "Edge-01",
		SiteID:      uuid.New().String(),
	})

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateways", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var resp dto.RegisterGatewayResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if resp.MQTTPassword != "one-time-secret" {
		t.Errorf("expected one-time mqtt_password in response")
	}
}

func TestRegisterGateway_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateways", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeleteGateway_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, svc, _, _, _ := newTestHandler()
	svc.DeleteFn = func(_ context.Context, got uuid.UUID) error {
		if got != id {
			t.Errorf("expected id %s, got %s", id, got)
		}
		return nil
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/gateways/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestDeleteGateway_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/gateways/bad-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
