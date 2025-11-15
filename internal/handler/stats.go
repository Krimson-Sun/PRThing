package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"pr-service/internal/app/middleware"

	"go.uber.org/zap"
)

type prStatsService interface {
	GetAssignmentStats(ctx context.Context) (map[string]int, map[string]int, error)
}

// StatsHandler handles statistics endpoints
type StatsHandler struct {
	prService prStatsService
	logger    *zap.Logger
}

// NewStatsHandler creates a new stats handler
func NewStatsHandler(prService prStatsService, logger *zap.Logger) *StatsHandler {
	return &StatsHandler{
		prService: prService,
		logger:    logger,
	}
}

type assignmentStatsResponse struct {
	ByUser map[string]int `json:"by_user"`
	ByPR   map[string]int `json:"by_pr"`
}

// GetAssignmentStats returns assignment statistics
func (h *StatsHandler) GetAssignmentStats(w http.ResponseWriter, r *http.Request) {
	byUser, byPR, err := h.prService.GetAssignmentStats(r.Context())
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	response := assignmentStatsResponse{
		ByUser: byUser,
		ByPR:   byPR,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}
