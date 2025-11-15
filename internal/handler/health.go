package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler returns service readiness information.
type HealthHandler struct {
	startedAt time.Time
}

// NewHealthHandler creates a health handler instance.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{startedAt: time.Now()}
}

type healthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	UptimeSec int64  `json:"uptime_seconds"`
}

// Check responds with a basic health payload.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		UptimeSec: int64(time.Since(h.startedAt).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
