package pullrequest

import (
	"context"
	"strings"

	"pr-service/internal/db"
	"pr-service/internal/domain"
	"pr-service/internal/service/assignment"
)

type prRepository interface {
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

type userRepository interface {
	GetUser(ctx context.Context, userID string) (domain.User, error)
	GetTeamMembers(ctx context.Context, teamName string) ([]domain.User, error)
}

// Service handles pull request business logic
type Service struct {
	prRepo         prRepository
	userRepo       userRepository
	transactor     db.Transactioner
	assignStrategy *assignment.Strategy
}

// NewService creates a new PR service
func NewService(
	prRepo prRepository,
	userRepo userRepository,
	transactor db.Transactioner,
	assignStrategy *assignment.Strategy,
) *Service {
	return &Service{
		prRepo:         prRepo,
		userRepo:       userRepo,
		transactor:     transactor,
		assignStrategy: assignStrategy,
	}
}

// CreatePR creates PR and auto-assigns reviewers
func (s *Service) CreatePR(
	ctx context.Context,
	prID, prName, authorID string,
) (domain.PullRequest, error) {
	prID = strings.TrimSpace(prID)
	prName = strings.TrimSpace(prName)
	authorID = strings.TrimSpace(authorID)
	if prID == "" || prName == "" || authorID == "" {
		return domain.PullRequest{}, domain.ErrInvalidArgument
	}

	// Check if PR already exists
	exists, err := s.prRepo.PRExists(ctx, prID)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if exists {
		return domain.PullRequest{}, domain.ErrPRExists
	}

	// Get author and their team
	author, err := s.userRepo.GetUser(ctx, authorID)
	if err != nil {
		return domain.PullRequest{}, err
	}

	teamMembers, err := s.userRepo.GetTeamMembers(ctx, author.TeamName)
	if err != nil {
		return domain.PullRequest{}, err
	}

	team := domain.Team{TeamName: author.TeamName, Members: teamMembers}

	// Select reviewers
	reviewerIDs := s.assignStrategy.SelectReviewers(ctx, team, authorID)

	// Create PR
	pr := domain.NewPullRequest(prID, prName, authorID)
	pr.AssignedReviewers = reviewerIDs

	// Create PR and assign reviewers in transaction
	err = s.transactor.Do(ctx, func(txCtx context.Context) error {
		if err := s.prRepo.CreatePR(txCtx, pr); err != nil {
			return err
		}

		if len(reviewerIDs) > 0 {
			if err := s.prRepo.AssignReviewers(txCtx, prID, reviewerIDs); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return domain.PullRequest{}, err
	}

	return pr, nil
}

// MergePR marks PR as merged (idempotent)
func (s *Service) MergePR(ctx context.Context, prID string) (domain.PullRequest, error) {
	prID = strings.TrimSpace(prID)
	if prID == "" {
		return domain.PullRequest{}, domain.ErrInvalidArgument
	}

	pr, err := s.prRepo.GetPR(ctx, prID)
	if err != nil {
		return domain.PullRequest{}, err
	}

	// Merge is idempotent - if already merged, just return current state
	pr.Merge()

	if err := s.prRepo.UpdatePR(ctx, pr); err != nil {
		return domain.PullRequest{}, err
	}

	return pr, nil
}

// ReassignReviewer replaces reviewer with another from their team
func (s *Service) ReassignReviewer(
	ctx context.Context,
	prID, oldUserID string,
) (domain.PullRequest, string, error) {
	prID = strings.TrimSpace(prID)
	oldUserID = strings.TrimSpace(oldUserID)
	if prID == "" || oldUserID == "" {
		return domain.PullRequest{}, "", domain.ErrInvalidArgument
	}

	pr, err := s.prRepo.GetPR(ctx, prID)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	if !pr.CanReassign() {
		return domain.PullRequest{}, "", domain.ErrPRMerged
	}

	if !pr.IsReviewerAssigned(oldUserID) {
		return domain.PullRequest{}, "", domain.ErrNotAssigned
	}

	// Get old reviewer's team
	oldUser, err := s.userRepo.GetUser(ctx, oldUserID)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	teamMembers, err := s.userRepo.GetTeamMembers(ctx, oldUser.TeamName)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	team := domain.Team{TeamName: oldUser.TeamName, Members: teamMembers}

	// Exclude author and current reviewers
	excludeIDs := append(pr.AssignedReviewers, pr.AuthorID)

	newUserID, err := s.assignStrategy.SelectReplacementReviewer(ctx, team, excludeIDs)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	// Replace reviewer in transaction
	err = s.transactor.Do(ctx, func(txCtx context.Context) error {
		// Remove old reviewer
		if err := s.prRepo.RemoveReviewer(txCtx, prID, oldUserID); err != nil {
			return err
		}

		// Add new reviewer
		if err := s.prRepo.AddReviewer(txCtx, prID, newUserID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return domain.PullRequest{}, "", err
	}

	// Update domain model
	if err := pr.ReplaceReviewer(oldUserID, newUserID); err != nil {
		return domain.PullRequest{}, "", err
	}

	return pr, newUserID, nil
}

// GetPRsByReviewer returns PRs where user is assigned as reviewer
func (s *Service) GetPRsByReviewer(
	ctx context.Context,
	userID string,
) ([]domain.PullRequest, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, domain.ErrInvalidArgument
	}

	return s.prRepo.GetPRsByReviewer(ctx, userID)
}

// GetAssignmentStats returns statistics about reviewer assignments
func (s *Service) GetAssignmentStats(ctx context.Context) (map[string]int, map[string]int, error) {
	byUser, err := s.prRepo.GetAssignmentStatsByUser(ctx)
	if err != nil {
		return nil, nil, err
	}

	byPR, err := s.prRepo.GetAssignmentStatsByPR(ctx)
	if err != nil {
		return nil, nil, err
	}

	return byUser, byPR, nil
}
