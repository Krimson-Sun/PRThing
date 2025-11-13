package repository

import (
	"context"
	"fmt"

	"pr-service/internal/db"
	"pr-service/internal/domain"

	"github.com/georgysavva/scany/v2/pgxscan"
)

type userRepository struct {
	BaseRepository
}

func NewUserRepository(cm db.EngineFactory) UserRepository {
	return &userRepository{
		BaseRepository: NewBaseRepository(cm),
	}
}

func (r *userRepository) CreateOrUpdateUser(ctx context.Context, user domain.User) error {
	query := `
		INSERT INTO users (user_id, username, team_name, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) 
		DO UPDATE SET
			username = EXCLUDED.username,
			team_name = EXCLUDED.team_name,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.Engine(ctx).Exec(ctx, query,
		user.UserID, user.Username, user.TeamName, user.IsActive, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create or update user: %w", err)
	}
	return nil
}

// UpdateUser updates user information
func (r *userRepository) UpdateUser(ctx context.Context, user domain.User) error {
	query := `
		UPDATE users
		SET username = $2, team_name = $3, is_active = $4, updated_at = $5
		WHERE user_id = $1
	`
	tag, err := r.Engine(ctx).Exec(ctx, query,
		user.UserID, user.Username, user.TeamName, user.IsActive, user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *userRepository) GetUser(ctx context.Context, userID string) (domain.User, error) {
	query := `
		SELECT user_id, username, team_name, is_active, created_at, updated_at
		FROM users
		WHERE user_id = $1
	`
	var user domain.User
	err := pgxscan.Get(ctx, r.Engine(ctx), &user, query, userID)
	if err != nil {
		if pgxscan.NotFound(err) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (r *userRepository) GetTeamMembers(ctx context.Context, teamName string) ([]domain.User, error) {
	query := `
		SELECT user_id, username, team_name, is_active, created_at, updated_at
		FROM users
		WHERE team_name = $1
		ORDER BY username
	`
	var users []domain.User
	err := pgxscan.Select(ctx, r.Engine(ctx), &users, query, teamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	return users, nil
}
