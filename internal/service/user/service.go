package user

import (
	"context"

	"pr-service/internal/domain"
)

type userRepository interface {
	GetUser(ctx context.Context, userID string) (domain.User, error)
	UpdateUser(ctx context.Context, user domain.User) error
}

type prRepository interface {
	GetPRsByReviewer(ctx context.Context, userID string) ([]domain.PullRequest, error)
}

// Service handles user business logic
type Service struct {
	userRepo userRepository
	prRepo   prRepository
}

// NewService creates a new user service
func NewService(userRepo userRepository, prRepo prRepository) *Service {
	return &Service{
		userRepo: userRepo,
		prRepo:   prRepo,
	}
}

// SetIsActive updates user's active status
func (s *Service) SetIsActive(
	ctx context.Context,
	userID string,
	isActive bool,
) (domain.User, error) {
	user, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}

	user.SetIsActive(isActive)

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return domain.User{}, err
	}

	return user, nil
}

// GetUser retrieves a user by ID
func (s *Service) GetUser(ctx context.Context, userID string) (domain.User, error) {
	return s.userRepo.GetUser(ctx, userID)
}

// GetPRsByReviewer returns PRs where user is assigned as reviewer
func (s *Service) GetPRsByReviewer(
	ctx context.Context,
	userID string,
) ([]domain.PullRequest, error) {
	return s.prRepo.GetPRsByReviewer(ctx, userID)
}
