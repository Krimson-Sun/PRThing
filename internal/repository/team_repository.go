package repository

import (
	"context"
	"fmt"

	"pr-service/internal/db"
	"pr-service/internal/domain"

	"github.com/georgysavva/scany/v2/pgxscan"
)

type teamRepository struct {
	BaseRepository
}

// NewTeamRepository creates a new team repository
func NewTeamRepository(cm db.EngineFactory) TeamRepository {
	return &teamRepository{
		BaseRepository: NewBaseRepository(cm),
	}
}

// CreateTeam creates a new team
func (r *teamRepository) CreateTeam(ctx context.Context, team domain.Team) error {
	query := `
		INSERT INTO teams (team_name, created_at, updated_at)
		VALUES ($1, $2, $3)
	`
	_, err := r.Engine(ctx).Exec(ctx, query, team.TeamName, team.CreatedAt, team.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}
	return nil
}

// GetTeam retrieves a team with its members
func (r *teamRepository) GetTeam(ctx context.Context, teamName string) (domain.Team, error) {
	// First, check if team exists
	var team domain.Team
	teamQuery := `
		SELECT team_name, created_at, updated_at
		FROM teams
		WHERE team_name = $1
	`
	err := pgxscan.Get(ctx, r.Engine(ctx), &team, teamQuery, teamName)
	if err != nil {
		if pgxscan.NotFound(err) {
			return domain.Team{}, domain.ErrNotFound
		}
		return domain.Team{}, fmt.Errorf("failed to get team: %w", err)
	}

	// Get team members
	membersQuery := `
		SELECT user_id, username, team_name, is_active, created_at, updated_at
		FROM users
		WHERE team_name = $1
		ORDER BY username
	`
	var members []domain.User
	err = pgxscan.Select(ctx, r.Engine(ctx), &members, membersQuery, teamName)
	if err != nil {
		return domain.Team{}, fmt.Errorf("failed to get team members: %w", err)
	}

	team.Members = members
	return team, nil
}

// TeamExists checks if a team exists
func (r *teamRepository) TeamExists(ctx context.Context, teamName string) (bool, error) {
	query := `
		SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)
	`
	var exists bool
	err := r.Engine(ctx).QueryRow(ctx, query, teamName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check team existence: %w", err)
	}
	return exists, nil
}
