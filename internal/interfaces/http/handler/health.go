package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthChecker is an interface for services that can be health-checked
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles health check endpoints
type HealthHandler struct {
	dbClient    HealthChecker
	redisClient HealthChecker
	version     string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(dbClient, redisClient HealthChecker, version string) *HealthHandler {
	return &HealthHandler{
		dbClient:    dbClient,
		redisClient: redisClient,
		version:     version,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services,omitempty"`
}

// Health handles GET /health
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Version:   h.version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, response)
}

// Ready handles GET /ready
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	services := make(map[string]string)
	allHealthy := true

	// Check database
	if h.dbClient != nil {
		if err := h.dbClient.Ping(ctx); err != nil {
			services["database"] = "unhealthy: " + err.Error()
			allHealthy = false
		} else {
			services["database"] = "healthy"
		}
	}

	// Check Redis
	if h.redisClient != nil {
		if err := h.redisClient.Ping(ctx); err != nil {
			services["redis"] = "unhealthy: " + err.Error()
			allHealthy = false
		} else {
			services["redis"] = "healthy"
		}
	}

	response := HealthResponse{
		Version:   h.version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Services:  services,
	}

	if allHealthy {
		response.Status = "ready"
		writeJSON(w, http.StatusOK, response)
	} else {
		response.Status = "not ready"
		writeJSON(w, http.StatusServiceUnavailable, response)
	}
}

// Live handles GET /live
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

