package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hill/orion/internal/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// buildRouter creates a minimal test router that mirrors the production layout:
//   - GET  /api/v1/sites          (auth only, no extra permission required)
//   - POST /api/v1/sites          (auth + PermSiteWrite)
//   - DELETE /api/v1/sites/:id    (auth + PermSiteDelete)
func buildRouter(authn middleware.Authenticator) *gin.Engine {
	r := gin.New()

	v1 := r.Group("/api/v1", middleware.Auth(authn))
	{
		sites := v1.Group("/sites")
		sites.GET("", func(c *gin.Context) { c.Status(http.StatusOK) })
		sites.POST("", middleware.RequirePermission(middleware.PermSiteWrite),
			func(c *gin.Context) { c.Status(http.StatusOK) })
		sites.DELETE("/:id", middleware.RequirePermission(middleware.PermSiteDelete),
			func(c *gin.Context) { c.Status(http.StatusOK) })
	}

	return r
}

// doRequest is a small helper that fires a request against r and returns the
// recorded response code.
func doRequest(r *gin.Engine, method, path, token string) int {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// ── No token → 401 ────────────────────────────────────────────────────────────

func TestAuth_NoToken_Returns401(t *testing.T) {
	authn := middleware.NewStubAuthenticator()
	r := buildRouter(authn)

	code := doRequest(r, http.MethodGet, "/api/v1/sites", "")
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

// ── Invalid token → 401 ───────────────────────────────────────────────────────

func TestAuth_InvalidToken_Returns401(t *testing.T) {
	authn := middleware.NewStubAuthenticator()
	r := buildRouter(authn)

	code := doRequest(r, http.MethodGet, "/api/v1/sites", "not-a-valid-token")
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

// ── dev-visitor DELETE /sites/:id → 403 ───────────────────────────────────────

func TestAuth_Visitor_DeleteSite_Returns403(t *testing.T) {
	authn := middleware.NewStubAuthenticator()
	r := buildRouter(authn)

	id := uuid.New().String()
	code := doRequest(r, http.MethodDelete, "/api/v1/sites/"+id, "dev-visitor")
	if code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", code)
	}
}

// ── dev-admin DELETE /sites/:id → not 401/403 (auth layer passes) ─────────────

func TestAuth_Admin_DeleteSite_PassesAuth(t *testing.T) {
	authn := middleware.NewStubAuthenticator()
	r := buildRouter(authn)

	id := uuid.New().String()
	code := doRequest(r, http.MethodDelete, "/api/v1/sites/"+id, "dev-admin")
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		t.Errorf("expected auth layer to pass, got %d", code)
	}
}

// ── dev-owner POST /sites → not 401/403 (auth layer passes) ───────────────────

func TestAuth_Owner_CreateSite_PassesAuth(t *testing.T) {
	authn := middleware.NewStubAuthenticator()
	r := buildRouter(authn)

	code := doRequest(r, http.MethodPost, "/api/v1/sites", "dev-owner")
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		t.Errorf("expected auth layer to pass, got %d", code)
	}
}

// ── HasPermission unit tests ───────────────────────────────────────────────────

func TestIdentity_HasPermission(t *testing.T) {
	id := &middleware.Identity{
		UserID:      "u1",
		Roles:       []string{"owner"},
		Permissions: []string{middleware.PermSiteRead, middleware.PermSiteWrite},
	}

	if !id.HasPermission(middleware.PermSiteRead) {
		t.Error("expected HasPermission(PermSiteRead) == true")
	}
	if !id.HasPermission(middleware.PermSiteWrite) {
		t.Error("expected HasPermission(PermSiteWrite) == true")
	}
	if id.HasPermission(middleware.PermSiteDelete) {
		t.Error("expected HasPermission(PermSiteDelete) == false")
	}
}

// ── StubAuthenticator role→permission coverage ────────────────────────────────

func TestStubAuthenticator_AdminHasAllPerms(t *testing.T) {
	authn := middleware.NewStubAuthenticator()
	id, err := authn.Authenticate(t.Context(), "dev-admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allPerms := []string{
		middleware.PermSiteRead, middleware.PermSiteWrite, middleware.PermSiteDelete,
		middleware.PermGatewayRead, middleware.PermGatewayWrite, middleware.PermGatewayDelete,
		middleware.PermTelemetryRead, middleware.PermDashboardManage,
		middleware.PermTopologyRead, middleware.PermUserManage,
	}
	for _, p := range allPerms {
		if !id.HasPermission(p) {
			t.Errorf("admin should have permission %q", p)
		}
	}
}

func TestStubAuthenticator_VisitorOnlyHasSiteRead(t *testing.T) {
	authn := middleware.NewStubAuthenticator()
	id, err := authn.Authenticate(t.Context(), "dev-visitor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !id.HasPermission(middleware.PermSiteRead) {
		t.Error("visitor should have site:read")
	}
	if id.HasPermission(middleware.PermSiteWrite) {
		t.Error("visitor should NOT have site:write")
	}
}
