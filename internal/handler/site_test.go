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

// ── Sites ─────────────────────────────────────────────────────────────────────

func TestListSites_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, svc, _ := newTestHandler()
	svc.ListFn = func(_ context.Context) ([]dto.SiteResponse, error) {
		return []dto.SiteResponse{
			{ID: uuid.MustParse("835bf184-4d97-4fcc-be26-d746d61a8020"), UtilityID: "A001", NameCN: "測試廠", SiteCode: "test"},
		}, nil
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sites", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []dto.SiteResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if len(body) != 1 || body[0].UtilityID != "A001" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestListSites_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, svc, _ := newTestHandler()
	svc.ListFn = func(_ context.Context) ([]dto.SiteResponse, error) {
		return nil, errors.New("db error")
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sites", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetSite_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, _, _, svc, _ := newTestHandler()
	svc.GetByIDFn = func(_ context.Context, got uuid.UUID) (*dto.SiteResponse, error) {
		if got != id {
			t.Errorf("expected id %s, got %s", id, got)
		}
		return &dto.SiteResponse{ID: id, UtilityID: "A001"}, nil
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sites/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetSite_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sites/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetSite_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, _, _, svc, _ := newTestHandler()
	svc.GetByIDFn = func(_ context.Context, _ uuid.UUID) (*dto.SiteResponse, error) {
		return nil, apperr.ErrNotFound
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sites/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateSite_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, svc, _ := newTestHandler()
	svc.CreateFn = func(_ context.Context, req dto.CreateSiteRequest) (*dto.SiteResponse, error) {
		return &dto.SiteResponse{ID: uuid.New(), UtilityID: req.UtilityID, NameCN: req.NameCN, SiteCode: req.SiteCode}, nil
	}

	body, _ := json.Marshal(dto.CreateSiteRequest{UtilityID: "A001", NameCN: "測試廠", SiteCode: "test"})
	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sites", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestCreateSite_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sites", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeleteSite_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	h, _, _, _, svc, _ := newTestHandler()
	svc.DeleteFn = func(_ context.Context, got uuid.UUID) error {
		if got != id {
			t.Errorf("expected id %s, got %s", id, got)
		}
		return nil
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sites/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

// ── Zones ─────────────────────────────────────────────────────────────────────

func TestListZones_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	siteID := uuid.New()
	h, _, _, _, _, zoneSvc := newTestHandler()
	zoneSvc.ListFn = func(_ context.Context, got uuid.UUID) ([]dto.ZoneResponse, error) {
		if got != siteID {
			t.Errorf("expected siteID %s, got %s", siteID, got)
		}
		return []dto.ZoneResponse{
			{ID: uuid.New(), SiteID: siteID, ZoneName: "B1機房", DisplayOrder: 1},
		}, nil
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sites/"+siteID.String()+"/zones", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []dto.ZoneResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if len(body) != 1 || body[0].ZoneName != "B1機房" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestListZones_InvalidSiteUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h, _, _, _, _, _ := newTestHandler()
	r := h.SetupRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sites/bad-id/zones", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateZone_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	siteID := uuid.New()
	h, _, _, _, _, zoneSvc := newTestHandler()
	zoneSvc.CreateFn = func(_ context.Context, got uuid.UUID, req dto.CreateZoneRequest) (*dto.ZoneResponse, error) {
		return &dto.ZoneResponse{ID: uuid.New(), SiteID: got, ZoneName: req.ZoneName}, nil
	}

	body, _ := json.Marshal(dto.CreateZoneRequest{ZoneName: "屋頂", DisplayOrder: 2})
	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sites/"+siteID.String()+"/zones", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestDeleteZone_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	siteID := uuid.New()
	zoneID := uuid.New()
	h, _, _, _, _, zoneSvc := newTestHandler()
	zoneSvc.DeleteFn = func(_ context.Context, gotSite, gotZone uuid.UUID) error {
		if gotSite != siteID || gotZone != zoneID {
			t.Errorf("unexpected ids: site=%s zone=%s", gotSite, gotZone)
		}
		return nil
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sites/"+siteID.String()+"/zones/"+zoneID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestDeleteZone_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	siteID := uuid.New()
	zoneID := uuid.New()
	h, _, _, _, _, zoneSvc := newTestHandler()
	zoneSvc.DeleteFn = func(_ context.Context, _, _ uuid.UUID) error {
		return apperr.ErrNotFound
	}

	r := h.SetupRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sites/"+siteID.String()+"/zones/"+zoneID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
