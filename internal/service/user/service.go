package user

import (
	"context"
	"slices"
	"strings"

	"pr-service/internal/db"
	"pr-service/internal/domain"
	"pr-service/internal/service/assignment"
)

type userRepository interface {
	GetUser(ctx context.Context, userID string) (domain.User, error)
	UpdateUser(ctx context.Context, user domain.User) error
	GetTeamMembers(ctx context.Context, teamName string) ([]domain.User, error)
	DeactivateUsers(ctx context.Context, teamName string, userIDs []string) error
}

type prRepository interface {
	GetPRsByReviewer(ctx context.Context, userID string) ([]domain.PullRequest, error)
	GetOpenPRIDsByReviewer(ctx context.Context, userID string) ([]string, error)
	GetPR(ctx context.Context, prID string) (domain.PullRequest, error)
	RemoveReviewer(ctx context.Context, prID string, userID string) error
	AddReviewer(ctx context.Context, prID string, userID string) error
}

// Service handles user business logic
type Service struct {
	userRepo       userRepository
	prRepo         prRepository
	transactor     db.Transactioner
	assignStrategy *assignment.Strategy
}

// NewService creates a new user service
func NewService(
	userRepo userRepository,
	prRepo prRepository,
	transactor db.Transactioner,
	assignStrategy *assignment.Strategy,
) *Service {
	return &Service{
		userRepo:       userRepo,
		prRepo:         prRepo,
		transactor:     transactor,
		assignStrategy: assignStrategy,
	}
}

// SetIsActive updates user's active status
func (s *Service) SetIsActive(
	ctx context.Context,
	userID string,
	isActive bool,
) (domain.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.User{}, domain.ErrInvalidArgument
	}

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
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.User{}, domain.ErrInvalidArgument
	}

	return s.userRepo.GetUser(ctx, userID)
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

// BulkDeactivateTeamMembers deactivates users of a team and reassigns their open reviews.
func (s *Service) BulkDeactivateTeamMembers(
	ctx context.Context,
	teamName string,
	userIDs []string,
) (domain.Team, []string, []domain.Reassignment, error) {
	teamName = strings.TrimSpace(teamName)
	if teamName == "" || len(userIDs) == 0 {
		return domain.Team{}, nil, nil, domain.ErrInvalidArgument
	}

	normalized := make([]string, 0, len(userIDs))
	seen := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return domain.Team{}, nil, nil, domain.ErrInvalidArgument
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	members, err := s.userRepo.GetTeamMembers(ctx, teamName)
	if err != nil {
		return domain.Team{}, nil, nil, err
	}
	if len(members) == 0 {
		return domain.Team{}, nil, nil, domain.ErrNotFound
	}

	team := domain.Team{
		TeamName: teamName,
		Members:  members,
	}

	memberByID := make(map[string]*domain.User, len(team.Members))
	for i := range team.Members {
		member := &team.Members[i]
		memberByID[member.UserID] = member
	}

	targets := make([]domain.User, 0, len(normalized))
	for _, id := range normalized {
		member, ok := memberByID[id]
		if !ok {
			return domain.Team{}, nil, nil, domain.ErrNotFound
		}
		if !member.IsActive {
			continue
		}
		targets = append(targets, *member)
	}

	if len(targets) == 0 {
		return team, []string{}, []domain.Reassignment{}, nil
	}

	targetIDs := make([]string, len(targets))
	for i, u := range targets {
		targetIDs[i] = u.UserID
	}
	deactivated := slices.Clone(targetIDs)

	futureTeam := domain.Team{
		TeamName: team.TeamName,
		Members:  make([]domain.User, len(team.Members)),
	}
	copy(futureTeam.Members, team.Members)
	for i := range futureTeam.Members {
		if _, ok := seen[futureTeam.Members[i].UserID]; ok {
			futureTeam.Members[i].IsActive = false
		}
	}

	var reassignments []domain.Reassignment

	err = s.transactor.Do(ctx, func(txCtx context.Context) error {
		if err := s.userRepo.DeactivateUsers(txCtx, teamName, targetIDs); err != nil {
			return err
		}

		for _, target := range targets {
			prIDs, err := s.prRepo.GetOpenPRIDsByReviewer(txCtx, target.UserID)
			if err != nil {
				return err
			}

			for _, prID := range prIDs {
				pr, err := s.prRepo.GetPR(txCtx, prID)
				if err != nil {
					return err
				}

				if pr.IsMerged() {
					continue
				}

				exclude := slices.Clone(pr.AssignedReviewers)
				exclude = append(exclude, pr.AuthorID)

				newUserID, err := s.assignStrategy.SelectReplacementReviewer(txCtx, futureTeam, exclude)
				if err != nil {
					return err
				}

				if err := s.prRepo.RemoveReviewer(txCtx, prID, target.UserID); err != nil {
					return err
				}

				if err := s.prRepo.AddReviewer(txCtx, prID, newUserID); err != nil {
					return err
				}

				if err := pr.ReplaceReviewer(target.UserID, newUserID); err != nil {
					return err
				}

				reassignments = append(reassignments, domain.Reassignment{
					PullRequestID: prID,
					OldUserID:     target.UserID,
					NewUserID:     newUserID,
				})
			}
		}

		return nil
	})

	if err != nil {
		return domain.Team{}, nil, nil, err
	}

	for i := range team.Members {
		if _, ok := seen[team.Members[i].UserID]; ok {
			team.Members[i].IsActive = false
		}
	}

	return team, deactivated, reassignments, nil
}
