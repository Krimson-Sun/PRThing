package team

import (
	"context"
	"strings"

	"pr-service/internal/db"
	"pr-service/internal/domain"
)

type teamRepository interface {
	CreateTeam(ctx context.Context, team domain.Team) error
	GetTeam(ctx context.Context, teamName string) (domain.Team, error)
	TeamExists(ctx context.Context, teamName string) (bool, error)
}

type userRepository interface {
	CreateOrUpdateUser(ctx context.Context, user domain.User) error
}

// Service handles team business logic
type Service struct {
	teamRepo   teamRepository
	userRepo   userRepository
	transactor db.Transactioner
}

// NewService creates a new team service
func NewService(
	teamRepo teamRepository,
	userRepo userRepository,
	transactor db.Transactioner,
) *Service {
	return &Service{
		teamRepo:   teamRepo,
		userRepo:   userRepo,
		transactor: transactor,
	}
}

// CreateTeam creates a team with members in a transaction
func (s *Service) CreateTeam(
	ctx context.Context,
	teamName string,
	members []domain.User,
) (domain.Team, error) {
	teamName = strings.TrimSpace(teamName)
	if teamName == "" || len(members) == 0 {
		return domain.Team{}, domain.ErrInvalidArgument
	}

	for i := range members {
		members[i].UserID = strings.TrimSpace(members[i].UserID)
		members[i].Username = strings.TrimSpace(members[i].Username)
		members[i].TeamName = strings.TrimSpace(members[i].TeamName)

		if members[i].UserID == "" || members[i].Username == "" {
			return domain.Team{}, domain.ErrInvalidArgument
		}
		if members[i].TeamName == "" {
			members[i].TeamName = teamName
		}
		if members[i].TeamName != teamName {
			return domain.Team{}, domain.ErrInvalidArgument
		}
	}

	// Check if team already exists
	exists, err := s.teamRepo.TeamExists(ctx, teamName)
	if err != nil {
		return domain.Team{}, err
	}
	if exists {
		return domain.Team{}, domain.ErrTeamExists
	}

	team := domain.NewTeam(teamName, members)

	// Create team and upsert users in transaction
	err = s.transactor.Do(ctx, func(txCtx context.Context) error {
		// Create team
		if err := s.teamRepo.CreateTeam(txCtx, team); err != nil {
			return err
		}

		// Upsert all members
		for _, member := range members {
			if err := s.userRepo.CreateOrUpdateUser(txCtx, member); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return domain.Team{}, err
	}

	return team, nil
}

// GetTeam retrieves a team with its members
func (s *Service) GetTeam(ctx context.Context, teamName string) (domain.Team, error) {
	return s.teamRepo.GetTeam(ctx, teamName)
}
