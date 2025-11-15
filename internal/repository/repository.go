package repository

import (
	"context"

	"pr-service/internal/db"
	"pr-service/internal/domain"
)

// TeamRepository defines methods for team data access
type TeamRepository interface {
	CreateTeam(ctx context.Context, team domain.Team) error
	GetTeam(ctx context.Context, teamName string) (domain.Team, error)
	TeamExists(ctx context.Context, teamName string) (bool, error)
}

// UserRepository defines methods for user data access
type UserRepository interface {
	CreateOrUpdateUser(ctx context.Context, user domain.User) error
	UpdateUser(ctx context.Context, user domain.User) error
	GetUser(ctx context.Context, userID string) (domain.User, error)
	GetTeamMembers(ctx context.Context, teamName string) ([]domain.User, error)
}

type PRRepository interface {
	CreatePR(ctx context.Context, pr domain.PullRequest) error
	GetPR(ctx context.Context, prID string) (domain.PullRequest, error)
	UpdatePR(ctx context.Context, pr domain.PullRequest) error
	AssignReviewers(ctx context.Context, prID string, reviewers []string) error
	RemoveReviewer(ctx context.Context, prID string, userID string) error
	AddReviewer(ctx context.Context, prID string, userID string) error
	GetPRsByReviewer(ctx context.Context, userID string) ([]domain.PullRequest, error)
	PRExists(ctx context.Context, prID string) (bool, error)
	GetAssignmentStatsByUser(ctx context.Context) (map[string]int, error)
	GetAssignmentStatsByPR(ctx context.Context) (map[string]int, error)
}

type BaseRepository struct {
	cm db.EngineFactory
}

func NewBaseRepository(cm db.EngineFactory) BaseRepository {
	return BaseRepository{cm: cm}
}

func (r *BaseRepository) Engine(ctx context.Context) db.Engine {
	return r.cm.Get(ctx)
}
