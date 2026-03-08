package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/pkg/apperr"
)

// ── GET /api/v1/devices/:id/latest ───────────────────────────────────────────

func (h *Handler) LatestByDevice(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	data, err := h.TelemetrySvc.LatestByDevice(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no telemetry data found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// ── GET /api/v1/assignments/:id/latest ───────────────────────────────────────

func (h *Handler) LatestByAssignment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid assignment id"})
		return
	}

	data, err := h.TelemetrySvc.LatestByAssignment(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no telemetry data found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// ── GET /api/v1/devices/:id/history ──────────────────────────────────────────

func (h *Handler) HistoryByDevice(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	from, to, ok := parseTimeRange(c)
	if !ok {
		return
	}

	data, err := h.TelemetrySvc.HistoryByDevice(c.Request.Context(), id, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// ── GET /api/v1/assignments/:id/history ──────────────────────────────────────

func (h *Handler) HistoryByAssignment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid assignment id"})
		return
	}

	from, to, ok := parseTimeRange(c)
	if !ok {
		return
	}

	data, err := h.TelemetrySvc.HistoryByAssignment(c.Request.Context(), id, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// ── GET /api/v1/sites/:id/latest ─────────────────────────────────────────────

func (h *Handler) LatestBySite(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site id"})
		return
	}

	data, err := h.TelemetrySvc.LatestBySite(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// parseTimeRange extracts and validates ?from=&to= query parameters.
// Defaults: from = 24 hours ago, to = now.
func parseTimeRange(c *gin.Context) (from, to time.Time, ok bool) {
	var q dto.HistoryQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range: " + err.Error()})
		return
	}

	now := time.Now().UTC()
	if q.From.IsZero() {
		q.From = now.Add(-24 * time.Hour)
	}
	if q.To.IsZero() {
		q.To = now
	}
	if q.From.After(q.To) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'from' must be before 'to'"})
		return
	}

	return q.From, q.To, true
}
