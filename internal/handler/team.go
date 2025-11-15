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
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type TeamDTO struct {
	TeamName string          `json:"team_name"`
	Members  []TeamMemberDTO `json:"members"`
}

type createTeamResponse struct {
	Team TeamDTO `json:"team"`
}

// AddTeam handles POST /team/add
func (h *TeamHandler) AddTeam(w http.ResponseWriter, r *http.Request) {
	var req TeamDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteErrorResponse(w, domain.ErrInvalidArgument, h.logger)
		return
	}

	if err := validateTeamRequest(req); err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	teamName := strings.TrimSpace(req.TeamName)

	// Map DTO to domain
	members := make([]domain.User, len(req.Members))
	for i, m := range req.Members {
		userID := strings.TrimSpace(m.UserID)
		username := strings.TrimSpace(m.Username)
		members[i] = domain.NewUser(userID, username, teamName, m.IsActive)
	}

	// Call service
	createdTeam, err := h.service.CreateTeam(r.Context(), teamName, members)
	if err != nil {
		middleware.WriteErrorResponse(w, err, h.logger)
		return
	}

	resp := createTeamResponse{Team: mapTeamToDTO(createdTeam)}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
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

	resp := mapTeamToDTO(team)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func mapTeamToDTO(team domain.Team) TeamDTO {
	members := make([]TeamMemberDTO, len(team.Members))
	for i, m := range team.Members {
		members[i] = TeamMemberDTO{
			UserID:   m.UserID,
			Username: m.Username,
			IsActive: m.IsActive,
		}
	}

	return TeamDTO{
		TeamName: team.TeamName,
		Members:  members,
	}
}

func validateTeamRequest(req TeamDTO) error {
	teamName := strings.TrimSpace(req.TeamName)
	if teamName == "" {
		return domain.ErrInvalidArgument
	}

	if len(req.Members) == 0 {
		return domain.ErrInvalidArgument
	}

	for _, member := range req.Members {
		if strings.TrimSpace(member.UserID) == "" ||
			strings.TrimSpace(member.Username) == "" {
			return domain.ErrInvalidArgument
		}
	}

	return nil
}
