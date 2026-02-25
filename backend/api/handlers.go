package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ocohen/ocp-user-auditter/backend/db"
	"github.com/ocohen/ocp-user-auditter/backend/models"
)

type Handler struct {
	DB *db.DB
}

func NewHandler(database *db.DB) *Handler {
	return &Handler{DB: database}
}

func (h *Handler) Ingest(c *gin.Context) {
	var events []models.IngestPayload
	if err := c.ShouldBindJSON(&events); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(events) == 0 {
		c.JSON(http.StatusOK, gin.H{"inserted": 0})
		return
	}

	if err := h.DB.InsertEvents(c.Request.Context(), events); err != nil {
		log.Printf("Insert error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"inserted": len(events)})
}

func (h *Handler) ListEvents(c *gin.Context) {
	var q models.EventQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.DB.QueryEvents(c.Request.Context(), q)
	if err != nil {
		log.Printf("Query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetEvent(c *gin.Context) {
	id := c.Param("id")
	event, err := h.DB.GetEvent(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}

	c.JSON(http.StatusOK, event)
}

func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.DB.GetStats(c.Request.Context())
	if err != nil {
		log.Printf("Stats error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stats failed"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handler) Healthz(c *gin.Context) {
	if err := h.DB.Pool.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}
