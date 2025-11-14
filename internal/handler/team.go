package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"pr-service/internal/app/middleware"
	"pr-service/internal/domain"

	"go.uber.org/zap"
)

type teamService interface {
	CreateTeam(ctx context.Context, teamName string, members []domain.User) (domain.Team, error)
	GetTeam(ctx context.Context, teamName string) (domain.Team, error)
}

// TeamHandler handles team-related HTTP requests
type TeamHandler struct {
	service teamService
	logger  *zap.Logger
}

// NewTeamHandler creates a new team handler
func NewTeamHandler(service teamService, logger *zap.Logger) *TeamHandler {
	return &TeamHandler{
		service: service,
		logger:  logger,
	}
}

// Team DTOs matching OpenAPI schema with snake_case

type TeamMemberDTO struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

type TeamDTO struct {
	TeamName string          `json:"team_name"`
	Members  []TeamMemberDTO `json:"members"`
}

type TeamResponse struct {
	TeamName string          `json:"team_name"`
	Members  []TeamMemberDTO `json:"members"`
}

// AddTeam handles POST /team/add
func (h *TeamHandler) AddTeam(w http.ResponseWriter, r *http.Request) {
	var req TeamDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	// Map DTO to domain
	members := make([]domain.User, len(req.Members))
	for i, m := range req.Members {
		members[i] = domain.User{
			UserID:   m.UserID,
			TeamName: req.TeamName,
			IsActive: m.IsActive,
		}
	}

	// Call service
	createdTeam, err := h.service.CreateTeam(r.Context(), req.TeamName, members)
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	// Build response (echo back the created team)
	resp := TeamResponse{
		TeamName: createdTeam.TeamName,
		Members:  req.Members,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// GetTeam handles GET /team/get?team_name=...
func (h *TeamHandler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	team, err := h.service.GetTeam(r.Context(), teamName)
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	// Map domain to DTO
	members := make([]TeamMemberDTO, len(team.Members))
	for i, m := range team.Members {
		members[i] = TeamMemberDTO{
			UserID:   m.UserID,
			IsActive: m.IsActive,
		}
	}

	resp := TeamResponse{
		TeamName: team.TeamName,
		Members:  members,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
