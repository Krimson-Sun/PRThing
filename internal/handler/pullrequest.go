package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"pr-service/internal/app/middleware"
	"pr-service/internal/domain"

	"go.uber.org/zap"
)

type prService interface {
	CreatePR(ctx context.Context, prID, prName, authorID string) (domain.PullRequest, error)
	MergePR(ctx context.Context, prID string) (domain.PullRequest, error)
	ReassignReviewer(ctx context.Context, prID, oldUserID string) (domain.PullRequest, string, error)
}

// PRHandler handles pull request HTTP requests
type PRHandler struct {
	service prService
	logger  *zap.Logger
}

// NewPRHandler creates a new PR handler
func NewPRHandler(service prService, logger *zap.Logger) *PRHandler {
	return &PRHandler{
		service: service,
		logger:  logger,
	}
}

// PR DTOs matching OpenAPI schema with snake_case

type CreatePRRequest struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}

type MergePRRequest struct {
	PullRequestID string `json:"pull_request_id"`
}

type ReassignRequest struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"` // per OpenAPI schema (not old_reviewer_id)
}

type PullRequestDTO struct {
	PullRequestID     string   `json:"pull_request_id"`
	PullRequestName   string   `json:"pull_request_name"`
	AuthorID          string   `json:"author_id"`
	AssignedReviewers []string `json:"assigned_reviewers"`
	Status            string   `json:"status"`
	CreatedAt         *string  `json:"createdAt,omitempty"`
	MergedAt          *string  `json:"mergedAt,omitempty"`
}

type prEnvelope struct {
	PR PullRequestDTO `json:"pr"`
}

type ReassignResponse struct {
	PR         PullRequestDTO `json:"pr"`
	ReplacedBy string         `json:"replaced_by"`
}

// CreatePR handles POST /pullRequest/create
func (h *PRHandler) CreatePR(w http.ResponseWriter, r *http.Request) {
	var req CreatePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	normalizeCreatePRRequest(&req)
	if err := validateCreatePRRequest(req); err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	pr, err := h.service.CreatePR(r.Context(), req.PullRequestID, req.PullRequestName, req.AuthorID)
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	resp := prEnvelope{PR: mapPRToDTO(pr)}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// MergePR handles POST /pullRequest/merge
func (h *PRHandler) MergePR(w http.ResponseWriter, r *http.Request) {
	var req MergePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	req.PullRequestID = strings.TrimSpace(req.PullRequestID)
	if err := validateMergeRequest(req); err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	pr, err := h.service.MergePR(r.Context(), req.PullRequestID)
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	resp := prEnvelope{PR: mapPRToDTO(pr)}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// ReassignReviewer handles POST /pullRequest/reassign
func (h *PRHandler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	var req ReassignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	normalizeReassignRequest(&req)
	if err := validateReassignRequest(req); err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	pr, replacedBy, err := h.service.ReassignReviewer(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	resp := ReassignResponse{
		PR:         mapPRToDTO(pr),
		ReplacedBy: replacedBy,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// Helper to map domain.PullRequest to DTO
func mapPRToDTO(pr domain.PullRequest) PullRequestDTO {
	dto := PullRequestDTO{
		PullRequestID:     pr.PullRequestID,
		PullRequestName:   pr.PullRequestName,
		AuthorID:          pr.AuthorID,
		AssignedReviewers: pr.AssignedReviewers,
		Status:            string(pr.Status),
	}

	// Handle nullable timestamps
	if !pr.CreatedAt.IsZero() {
		createdAtStr := pr.CreatedAt.Format(time.RFC3339)
		dto.CreatedAt = &createdAtStr
	}

	if pr.MergedAt != nil {
		mergedAtStr := pr.MergedAt.Format(time.RFC3339)
		dto.MergedAt = &mergedAtStr
	}

	return dto
}

func normalizeCreatePRRequest(req *CreatePRRequest) {
	req.PullRequestID = strings.TrimSpace(req.PullRequestID)
	req.PullRequestName = strings.TrimSpace(req.PullRequestName)
	req.AuthorID = strings.TrimSpace(req.AuthorID)
}

func validateCreatePRRequest(req CreatePRRequest) error {
	if req.PullRequestID == "" ||
		req.PullRequestName == "" ||
		req.AuthorID == "" {
		return domain.ErrInvalidArgument
	}
	return nil
}

func validateMergeRequest(req MergePRRequest) error {
	if req.PullRequestID == "" {
		return domain.ErrInvalidArgument
	}
	return nil
}

func normalizeReassignRequest(req *ReassignRequest) {
	req.PullRequestID = strings.TrimSpace(req.PullRequestID)
	req.OldUserID = strings.TrimSpace(req.OldUserID)
}

func validateReassignRequest(req ReassignRequest) error {
	if req.PullRequestID == "" || req.OldUserID == "" {
		return domain.ErrInvalidArgument
	}
	return nil
}
