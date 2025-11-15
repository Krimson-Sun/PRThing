package assignment

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"pr-service/internal/domain"
)

// Strategy implements reviewer selection algorithms
type Strategy struct {
	rng *rand.Rand
	mu  sync.Mutex
}

// NewStrategy creates a new assignment strategy
func NewStrategy() *Strategy {
	return &Strategy{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SelectReviewers selects up to 2 active reviewers from team, excluding author
func (s *Strategy) SelectReviewers(
	ctx context.Context,
	team domain.Team,
	authorID string,
) []string {
	candidates := team.GetActiveMembersExcluding(authorID)

	if len(candidates) == 0 {
		return []string{}
	}

	// Shuffle for randomness
	s.mu.Lock()
	s.rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	s.mu.Unlock()

	// Select up to 2
	maxReviewers := 2
	if len(candidates) < maxReviewers {
		maxReviewers = len(candidates)
	}

	reviewers := make([]string, maxReviewers)
	for i := 0; i < maxReviewers; i++ {
		reviewers[i] = candidates[i].UserID
	}

	return reviewers
}

// SelectReplacementReviewer selects replacement from same team, excluding current reviewers
func (s *Strategy) SelectReplacementReviewer(
	ctx context.Context,
	team domain.Team,
	excludeUserIDs []string,
) (string, error) {
	candidates := team.GetActiveMembers()

	// Filter out excluded users
	filtered := make([]domain.User, 0)
	for _, c := range candidates {
		excluded := false
		for _, exID := range excludeUserIDs {
			if c.UserID == exID {
				excluded = true
				break
			}
		}
		if !excluded {
			filtered = append(filtered, c)
		}
	}

	if len(filtered) == 0 {
		return "", domain.ErrNoCandidate
	}

	// Random selection
	s.mu.Lock()
	idx := s.rng.Intn(len(filtered))
	s.mu.Unlock()
	return filtered[idx].UserID, nil
}
