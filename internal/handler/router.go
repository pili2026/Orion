// Package handler sets up the HTTP router for the application using the Gin framework.
package handler

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"

	"github.com/hill/orion/internal/database"
)

// Handler holds every dependency that HTTP and MQTT handlers need.
type Handler struct {
	DB         *database.DBManager
	MQTT       mqtt.Client
	GatewaySvc GatewayService
}

// NewHandler creates a new Handler with the provided dependencies.
func NewHandler(db *database.DBManager, mqttClient mqtt.Client, gatewaySvc GatewayService) *Handler {
	return &Handler{
		DB:         db,
		MQTT:       mqttClient,
		GatewaySvc: gatewaySvc,
	}
}

// SetupRouter registers all HTTP routes and returns the engine.
func (h *Handler) SetupRouter() *gin.Engine {
	r := gin.Default()

	// TRUSTED_PROXIES: comma-separated IPs/CIDRs of your reverse proxy.
	// Set to 127.0.0.1 when Nginx runs on the same machine as Orion.
	if raw := os.Getenv("TRUSTED_PROXIES"); raw != "" {
		proxies := strings.Split(raw, ",")
		for i := range proxies {
			proxies[i] = strings.TrimSpace(proxies[i])
		}
		if err := r.SetTrustedProxies(proxies); err != nil {
			slog.Error("Failed to set trusted proxies", slog.Any("error", err))
		}
	} else {
		if err := r.SetTrustedProxies([]string{}); err != nil {
			slog.Error("Failed to disable trusted proxies", slog.Any("error", err))
		}
		slog.Warn("TRUSTED_PROXIES not set — proxy headers will be ignored")
	}

	// ── Public ───────────────────────────────────────────────────────────────
	r.GET("/health", h.healthCheck)

	// ── API v1 ───────────────────────────────────────────────────────────────
	// Add auth middleware here once implemented:
	//   v1 := r.Group("/api/v1", middleware.Auth())
	v1 := r.Group("/api/v1")
	{
		gateways := v1.Group("/gateways")
		{
			gateways.POST("", h.RegisterGateway)
			gateways.GET("", h.ListGateways)
			gateways.GET("/:id", h.GetGateway)
			gateways.PATCH("/:id", h.UpdateGateway)
			gateways.DELETE("/:id", h.DeleteGateway)
		}
	}

	return r
}

func (h *Handler) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
