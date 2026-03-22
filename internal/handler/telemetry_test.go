package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/pkg/apperr"
)

// ── Device latest ─────────────────────────────────────────────────────────────

func TestGetDeviceLatest_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, _, svc, _, _ := newTestHandler()
	svc.LatestByDeviceFn = func(_ context.Context, got uuid.UUID) (any, error) {
		if got != id {
			t.Errorf("expected id %s, got %s", id, got)
		}
		return map[string]any{"device_id": id.String(), "kw": 12.5}, nil
	}

	r := h.SetupRouter()
	req := authedReq(http.MethodGet, "/api/v1/devices/"+id.String()+"/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetDeviceLatest_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, _, svc, _, _ := newTestHandler()
	svc.LatestByDeviceFn = func(_ context.Context, _ uuid.UUID) (any, error) {
		return nil, apperr.ErrNotFound
	}

	r := h.SetupRouter()
	req := authedReq(http.MethodGet, "/api/v1/devices/"+id.String()+"/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetDeviceLatest_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := authedReq(http.MethodGet, "/api/v1/devices/not-a-uuid/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── Device history ────────────────────────────────────────────────────────────

func TestGetDeviceHistory_DefaultsTo24h(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	var capturedFrom, capturedTo time.Time

	h, _, _, svc, _, _ := newTestHandler()
	svc.HistoryByDeviceFn = func(_ context.Context, _ uuid.UUID, from, to time.Time) (any, error) {
		capturedFrom = from
		capturedTo = to
		return []any{}, nil
	}

	r := h.SetupRouter()
	req := authedReq(http.MethodGet, "/api/v1/devices/"+id.String()+"/history", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	diff := capturedTo.Sub(capturedFrom)
	if diff < 23*time.Hour || diff > 25*time.Hour {
		t.Errorf("expected ~24h default range, got %v", diff)
	}
}

func TestGetDeviceHistory_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := authedReq(http.MethodGet, "/api/v1/devices/bad/history", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetDeviceHistory_FromAfterTo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := authedReq(http.MethodGet,
		"/api/v1/devices/"+id.String()+"/history?from=2024-06-01&to=2024-01-01", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for from > to, got %d", w.Code)
	}
}

// ── Assignment latest ─────────────────────────────────────────────────────────

func TestGetAssignmentLatest_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	ts := time.Now()
	val := 23.5
	h, _, _, svc, _, _ := newTestHandler()
	svc.LatestByAssignmentFn = func(_ context.Context, got uuid.UUID) (*dto.LatestSensorResponse, error) {
		return &dto.LatestSensorResponse{AssignmentID: got, TS: ts, Val: &val}, nil
	}

	r := h.SetupRouter()
	req := authedReq(http.MethodGet, "/api/v1/assignments/"+id.String()+"/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body dto.LatestSensorResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Val == nil || *body.Val != 23.5 {
		t.Errorf("expected val 23.5, got %v", body.Val)
	}
}

func TestGetAssignmentLatest_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, _, svc, _, _ := newTestHandler()
	svc.LatestByAssignmentFn = func(_ context.Context, _ uuid.UUID) (*dto.LatestSensorResponse, error) {
		return nil, apperr.ErrNotFound
	}

	r := h.SetupRouter()
	req := authedReq(http.MethodGet, "/api/v1/assignments/"+id.String()+"/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetAssignmentLatest_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := authedReq(http.MethodGet, "/api/v1/assignments/not-a-uuid/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetAssignmentLatest_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, _, svc, _, _ := newTestHandler()
	svc.LatestByAssignmentFn = func(_ context.Context, _ uuid.UUID) (*dto.LatestSensorResponse, error) {
		return nil, errors.New("db error")
	}

	r := h.SetupRouter()
	req := authedReq(http.MethodGet, "/api/v1/assignments/"+id.String()+"/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// ── Site latest ───────────────────────────────────────────────────────────────

func TestGetSiteLatest_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	siteID := uuid.New()
	h, _, _, svc, _, _ := newTestHandler()
	svc.LatestBySiteFn = func(_ context.Context, got uuid.UUID) (*dto.SiteLatestResponse, error) {
		return &dto.SiteLatestResponse{SiteID: got, UtilityID: "A001"}, nil
	}

	r := h.SetupRouter()
	req := authedReq(http.MethodGet, "/api/v1/sites/"+siteID.String()+"/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetSiteLatest_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := authedReq(http.MethodGet, "/api/v1/sites/bad/latest", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
