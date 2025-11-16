package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"pr-service/internal/app/middleware"
	"pr-service/internal/domain"

	"go.uber.org/zap"
)

type userService interface {
	SetIsActive(ctx context.Context, userID string, isActive bool) (domain.User, error)
	GetPRsByReviewer(ctx context.Context, userID string) ([]domain.PullRequest, error)
	BulkDeactivateTeamMembers(ctx context.Context, teamName string, userIDs []string) (domain.Team, []string, []domain.Reassignment, error)
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
	Username string `json:"username"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

type PullRequestShort struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
	Status          string `json:"status"`
}

type setIsActiveResponse struct {
	User UserResponse `json:"user"`
}

type getReviewResponse struct {
	UserID       string             `json:"user_id"`
	PullRequests []PullRequestShort `json:"pull_requests"`
}

type BulkDeactivateRequest struct {
	TeamName string   `json:"team_name"`
	UserIDs  []string `json:"user_ids"`
}

type bulkDeactivateResponse struct {
	TeamName           string              `json:"team_name"`
	DeactivatedUserIDs []string            `json:"deactivated_user_ids"`
	Reassignments      []reassignmentDTO   `json:"reassignments"`
	TeamMembers        []bulkTeamMemberDTO `json:"team_members"`
}

type bulkTeamMemberDTO struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type reassignmentDTO struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"`
	NewUserID     string `json:"new_user_id"`
}

// SetIsActive handles POST /users/setIsActive
func (h *UserHandler) SetIsActive(w http.ResponseWriter, r *http.Request) {
	var req SetIsActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	req.UserID = strings.TrimSpace(req.UserID)
	if err := validateUserID(req.UserID); err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	user, err := h.service.SetIsActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	resp := setIsActiveResponse{User: mapUserToResponse(user)}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// GetReview handles GET /users/getReview?user_id=...
func (h *UserHandler) GetReview(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if err := validateUserID(userID); err != nil {
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

	resp := getReviewResponse{
		UserID:       userID,
		PullRequests: result,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func mapUserToResponse(user domain.User) UserResponse {
	return UserResponse{
		UserID:   user.UserID,
		Username: user.Username,
		TeamName: user.TeamName,
		IsActive: user.IsActive,
	}
}

func validateUserID(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return domain.ErrInvalidArgument
	}
	return nil
}

// BulkDeactivateTeamMembers handles POST /users/deactivateTeamMembers
func (h *UserHandler) BulkDeactivateTeamMembers(w http.ResponseWriter, r *http.Request) {
	var req BulkDeactivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	req.TeamName = strings.TrimSpace(req.TeamName)
	if req.TeamName == "" || len(req.UserIDs) == 0 {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	team, deactivated, reassignments, err := h.service.BulkDeactivateTeamMembers(r.Context(), req.TeamName, req.UserIDs)
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	resp := bulkDeactivateResponse{
		TeamName:           team.TeamName,
		DeactivatedUserIDs: deactivated,
		Reassignments:      make([]reassignmentDTO, len(reassignments)),
		TeamMembers:        make([]bulkTeamMemberDTO, len(team.Members)),
	}

	for i, member := range team.Members {
		resp.TeamMembers[i] = bulkTeamMemberDTO{
			UserID:   member.UserID,
			Username: member.Username,
			IsActive: member.IsActive,
		}
	}

	for i, reassignment := range reassignments {
		resp.Reassignments[i] = reassignmentDTO{
			PullRequestID: reassignment.PullRequestID,
			OldUserID:     reassignment.OldUserID,
			NewUserID:     reassignment.NewUserID,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
