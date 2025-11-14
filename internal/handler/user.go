package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"pr-service/internal/app/middleware"
	"pr-service/internal/domain"

	"go.uber.org/zap"
)

type userService interface {
	SetIsActive(ctx context.Context, userID string, isActive bool) (domain.User, error)
	GetPRsByReviewer(ctx context.Context, userID string) ([]domain.PullRequest, error)
}

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	service userService
	logger  *zap.Logger
}

// NewUserHandler creates a new user handler
func NewUserHandler(service userService, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		service: service,
		logger:  logger,
	}
}

// User DTOs matching OpenAPI schema with snake_case

type SetIsActiveRequest struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

type UserResponse struct {
	UserID   string `json:"user_id"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

type PullRequestShort struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
	Status          string `json:"status"`
}

// SetIsActive handles POST /users/setIsActive
func (h *UserHandler) SetIsActive(w http.ResponseWriter, r *http.Request) {
	var req SetIsActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	user, err := h.service.SetIsActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	resp := UserResponse{
		UserID:   user.UserID,
		TeamName: user.TeamName,
		IsActive: user.IsActive,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// GetReview handles GET /users/getReview?user_id=...
func (h *UserHandler) GetReview(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	prs, err := h.service.GetPRsByReviewer(r.Context(), userID)
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	// Map to short DTO (without assigned_reviewers, createdAt, mergedAt)
	result := make([]PullRequestShort, len(prs))
	for i, pr := range prs {
		result[i] = PullRequestShort{
			PullRequestID:   pr.PullRequestID,
			PullRequestName: pr.PullRequestName,
			AuthorID:        pr.AuthorID,
			Status:          string(pr.Status),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}
