package user

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"pr-service/internal/domain"
	"pr-service/internal/service/assignment"
)

type fakeUserRepo struct {
	users map[string]domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users: make(map[string]domain.User),
	}
}

func (r *fakeUserRepo) GetUser(ctx context.Context, userID string) (domain.User, error) {
	if user, ok := r.users[userID]; ok {
		return user, nil
	}
	return domain.User{}, domain.ErrNotFound
}

func (r *fakeUserRepo) UpdateUser(ctx context.Context, user domain.User) error {
	if _, ok := r.users[user.UserID]; !ok {
		return domain.ErrNotFound
	}
	user.UpdatedAt = time.Now()
	r.users[user.UserID] = user
	return nil
}

func (r *fakeUserRepo) GetTeamMembers(ctx context.Context, teamName string) ([]domain.User, error) {
	result := make([]domain.User, 0)
	for _, user := range r.users {
		if user.TeamName == teamName {
			result = append(result, user)
		}
	}
	if len(result) == 0 {
		return nil, domain.ErrNotFound
	}
	return result, nil
}

func (r *fakeUserRepo) DeactivateUsers(ctx context.Context, teamName string, userIDs []string) error {
	for _, id := range userIDs {
		user, ok := r.users[id]
		if !ok || user.TeamName != teamName {
			return domain.ErrNotFound
		}
		user.IsActive = false
		user.UpdatedAt = time.Now()
		r.users[id] = user
	}
	return nil
}

type fakePRRepo struct {
	prs map[string]domain.PullRequest
}

func newFakePRRepo() *fakePRRepo {
	return &fakePRRepo{
		prs: make(map[string]domain.PullRequest),
	}
}

func (r *fakePRRepo) GetPRsByReviewer(ctx context.Context, userID string) ([]domain.PullRequest, error) {
	result := make([]domain.PullRequest, 0)
	for _, pr := range r.prs {
		for _, reviewer := range pr.AssignedReviewers {
			if reviewer == userID {
				result = append(result, pr)
				break
			}
		}
	}
	return result, nil
}

func (r *fakePRRepo) GetOpenPRIDsByReviewer(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	for id, pr := range r.prs {
		if pr.Status != domain.PRStatusOpen {
			continue
		}
		for _, reviewer := range pr.AssignedReviewers {
			if reviewer == userID {
				ids = append(ids, id)
				break
			}
		}
	}
	return ids, nil
}

func (r *fakePRRepo) GetPR(ctx context.Context, prID string) (domain.PullRequest, error) {
	if pr, ok := r.prs[prID]; ok {
		return pr, nil
	}
	return domain.PullRequest{}, domain.ErrNotFound
}

func (r *fakePRRepo) RemoveReviewer(ctx context.Context, prID string, userID string) error {
	pr, ok := r.prs[prID]
	if !ok {
		return domain.ErrNotFound
	}

	found := false
	filtered := make([]string, 0, len(pr.AssignedReviewers))
	for _, reviewer := range pr.AssignedReviewers {
		if reviewer == userID {
			found = true
			continue
		}
		filtered = append(filtered, reviewer)
	}

	if !found {
		return domain.ErrNotFound
	}

	pr.AssignedReviewers = filtered
	r.prs[prID] = pr
	return nil
}

func (r *fakePRRepo) AddReviewer(ctx context.Context, prID string, userID string) error {
	pr, ok := r.prs[prID]
	if !ok {
		return domain.ErrNotFound
	}

	for _, reviewer := range pr.AssignedReviewers {
		if reviewer == userID {
			return nil
		}
	}

	pr.AssignedReviewers = append(pr.AssignedReviewers, userID)
	r.prs[prID] = pr
	return nil
}

type noopTransactor struct{}

func (noopTransactor) Do(ctx context.Context, f func(ctx context.Context) error) error {
	return f(ctx)
}

func TestBulkDeactivateTeamMembers(t *testing.T) {
	userRepo := newFakeUserRepo()
	prRepo := newFakePRRepo()

	userRepo.users["u1"] = domain.NewUser("u1", "Alice", "backend", true)
	userRepo.users["u2"] = domain.NewUser("u2", "Bob", "backend", true)
	userRepo.users["u3"] = domain.NewUser("u3", "Charlie", "backend", true)
	userRepo.users["u4"] = domain.NewUser("u4", "David", "backend", true)

	pr := domain.NewPullRequest("pr-1", "Add search", "u1")
	pr.AssignedReviewers = []string{"u2", "u3"}
	prRepo.prs["pr-1"] = pr

	strategy := assignment.NewStrategyWithSource(rand.NewSource(1))
	service := NewService(userRepo, prRepo, noopTransactor{}, strategy)

	team, deactivated, reassignments, err := service.BulkDeactivateTeamMembers(context.Background(), "backend", []string{"u2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deactivated) != 1 || deactivated[0] != "u2" {
		t.Fatalf("expected u2 to be deactivated, got %v", deactivated)
	}

	memberMap := make(map[string]domain.User)
	for _, member := range team.Members {
		memberMap[member.UserID] = member
	}
	if memberMap["u2"].IsActive {
		t.Fatalf("expected u2 to be inactive")
	}

	if len(reassignments) != 1 {
		t.Fatalf("expected one reassignment, got %d", len(reassignments))
	}

	if reassignments[0].OldUserID != "u2" {
		t.Fatalf("expected replacement for u2, got %s", reassignments[0].OldUserID)
	}

	if reassignments[0].NewUserID == "u2" {
		t.Fatalf("new reviewer must differ from old")
	}
}

func BenchmarkBulkDeactivateTeamMembers(b *testing.B) {
	for i := 0; i < b.N; i++ {
		userRepo := newFakeUserRepo()
		prRepo := newFakePRRepo()

		for u := 0; u < 20; u++ {
			id := fmt.Sprintf("u%d", u)
			userRepo.users[id] = domain.NewUser(id, fmt.Sprintf("User %d", u), "backend", true)
		}

		// Create 50 PRs with two reviewers each.
		for p := 0; p < 50; p++ {
			prID := fmt.Sprintf("pr-%d", p)
			pr := domain.NewPullRequest(prID, "Feature", "u0")
			pr.AssignedReviewers = []string{
				fmt.Sprintf("u%d", (p%18)+1),
				fmt.Sprintf("u%d", (p%18)+2),
			}
			prRepo.prs[prID] = pr
		}

		strategy := assignment.NewStrategyWithSource(rand.NewSource(42))
		service := NewService(userRepo, prRepo, noopTransactor{}, strategy)

		if _, _, _, err := service.BulkDeactivateTeamMembers(context.Background(), "backend", []string{"u1", "u2", "u3"}); err != nil {
			b.Fatalf("bulk deactivate failed: %v", err)
		}
	}
}
