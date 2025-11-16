package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"pr-service/internal/app/middleware"
	"pr-service/internal/domain"
	"pr-service/internal/handler"
	"pr-service/internal/service/assignment"
	"pr-service/internal/service/pullrequest"
	"pr-service/internal/service/team"
	"pr-service/internal/service/user"
)

func TestHTTPE2E(t *testing.T) {
	s := newTestServer(t)
	defer s.Close()

	teamPayload := map[string]any{
		"team_name": "backend",
		"members": []map[string]any{
			{"user_id": "u1", "username": "Alice", "is_active": true},
			{"user_id": "u2", "username": "Bob", "is_active": true},
			{"user_id": "u3", "username": "Charlie", "is_active": true},
			{"user_id": "u4", "username": "David", "is_active": true},
		},
	}

	var teamResp teamResponse
	s.postJSON("/team/add", teamPayload, http.StatusCreated, &teamResp)

	createPR := func(id, name, author string) createPRResponse {
		var resp createPRResponse
		s.postJSON("/pullRequest/create", map[string]string{
			"pull_request_id":   id,
			"pull_request_name": name,
			"author_id":         author,
		}, http.StatusCreated, &resp)
		return resp
	}

	pr1 := createPR("pr-1001", "Add search", "u1")
	pr2 := createPR("pr-1002", "Refactor payments", "u1")

	if len(pr2.PR.AssignedReviewers) == 0 {
		t.Fatalf("expected reviewers for pr-1002")
	}
	oldReviewer := pr2.PR.AssignedReviewers[0]

	var reassignResp reassignResponse
	s.postJSON("/pullRequest/reassign", map[string]string{
		"pull_request_id": "pr-1002",
		"old_user_id":     oldReviewer,
	}, http.StatusOK, &reassignResp)

	if reassignResp.ReplacedBy == oldReviewer {
		t.Fatalf("expected different reviewer after reassignment")
	}

	var mergeResp mergeResponse
	s.postJSON("/pullRequest/merge", map[string]string{"pull_request_id": "pr-1002"}, http.StatusOK, &mergeResp)
	s.postJSON("/pullRequest/merge", map[string]string{"pull_request_id": "pr-1002"}, http.StatusOK, &mergeResp)
	if mergeResp.PR.Status != "MERGED" {
		t.Fatalf("expected PR to be merged")
	}

	var stats statsResponse
	s.getJSON("/stats/assignments", http.StatusOK, &stats)
	if len(stats.ByUser) == 0 || len(stats.ByPR) == 0 {
		t.Fatalf("expected non-empty stats")
	}

	if len(pr1.PR.AssignedReviewers) == 0 {
		t.Fatalf("expected reviewers for pr-1001")
	}
	targetReviewer := pr1.PR.AssignedReviewers[0]

	var bulkResp bulkDeactivateResponse
	s.postJSON("/users/deactivateTeamMembers", map[string]any{
		"team_name": "backend",
		"user_ids":  []string{targetReviewer},
	}, http.StatusOK, &bulkResp)

	if len(bulkResp.Reassignments) == 0 {
		t.Fatalf("expected reassignment entries after deactivation")
	}

	reassignment := bulkResp.Reassignments[0]
	if reassignment.OldUserID != targetReviewer {
		t.Fatalf("expected reassignment for %s, got %s", targetReviewer, reassignment.OldUserID)
	}

	var oldReview getReviewResponse
	s.getJSON(fmt.Sprintf("/users/getReview?user_id=%s", targetReviewer), http.StatusOK, &oldReview)
	if containsPR(oldReview.PullRequests, "pr-1001") {
		t.Fatalf("expected pr-1001 to be removed from old reviewer")
	}

	var newReview getReviewResponse
	s.getJSON(fmt.Sprintf("/users/getReview?user_id=%s", reassignment.NewUserID), http.StatusOK, &newReview)
	if !containsPR(newReview.PullRequests, "pr-1001") {
		t.Fatalf("expected pr-1001 to be assigned to new reviewer %s", reassignment.NewUserID)
	}
}

type testServer struct {
	t      *testing.T
	server *httptest.Server
	client *http.Client
	base   string
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	userRepo := newMemoryUserRepo()
	teamRepo := newMemoryTeamRepo(userRepo)
	prRepo := newMemoryPRRepo()

	transactor := noopTransactor{}
	strategy := assignment.NewStrategyWithSource(rand.NewSource(1))

	teamService := team.NewService(teamRepo, userRepo, transactor)
	userService := user.NewService(userRepo, prRepo, transactor, strategy)
	prService := pullrequest.NewService(prRepo, userRepo, transactor, strategy)

	log := zap.NewNop()

	teamHandler := handler.NewTeamHandler(teamService, log)
	userHandler := handler.NewUserHandler(userService, log)
	prHandler := handler.NewPRHandler(prService, log)
	statsHandler := handler.NewStatsHandler(prService, log)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /team/add", teamHandler.AddTeam)
	mux.HandleFunc("GET /team/get", teamHandler.GetTeam)
	mux.HandleFunc("POST /users/setIsActive", userHandler.SetIsActive)
	mux.HandleFunc("GET /users/getReview", userHandler.GetReview)
	mux.HandleFunc("POST /users/deactivateTeamMembers", userHandler.BulkDeactivateTeamMembers)
	mux.HandleFunc("POST /pullRequest/create", prHandler.CreatePR)
	mux.HandleFunc("POST /pullRequest/merge", prHandler.MergePR)
	mux.HandleFunc("POST /pullRequest/reassign", prHandler.ReassignReviewer)
	mux.HandleFunc("GET /stats/assignments", statsHandler.GetAssignmentStats)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	var handler http.Handler = mux
	handler = middleware.Logging(log)(handler)
	handler = middleware.Recovery(log)(handler)

	server := httptest.NewServer(handler)

	return &testServer{
		t:      t,
		server: server,
		client: server.Client(),
		base:   server.URL,
	}
}

func (s *testServer) Close() {
	s.server.Close()
}

func (s *testServer) postJSON(path string, body any, expectedStatus int, out any) {
	s.t.Helper()

	var buf io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			s.t.Fatalf("failed to marshal request body: %v", err)
		}
		buf = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPost, s.base+path, buf)
	if err != nil {
		s.t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.t.Fatalf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(bodyBytes))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			s.t.Fatalf("failed to decode response: %v", err)
		}
	}
}

func (s *testServer) getJSON(path string, expectedStatus int, out any) {
	s.t.Helper()

	req, err := http.NewRequest(http.MethodGet, s.base+path, nil)
	if err != nil {
		s.t.Fatalf("failed to build request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.t.Fatalf("expected status %d, got %d: %s", expectedStatus, resp.StatusCode, string(bodyBytes))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			s.t.Fatalf("failed to decode response: %v", err)
		}
	}
}

type teamResponse struct {
	Team struct {
		TeamName string `json:"team_name"`
	} `json:"team"`
}

type createPRResponse struct {
	PR struct {
		PullRequestID     string   `json:"pull_request_id"`
		PullRequestName   string   `json:"pull_request_name"`
		AuthorID          string   `json:"author_id"`
		Status            string   `json:"status"`
		AssignedReviewers []string `json:"assigned_reviewers"`
	} `json:"pr"`
}

type reassignResponse struct {
	PR struct {
		PullRequestID     string   `json:"pull_request_id"`
		AssignedReviewers []string `json:"assigned_reviewers"`
	} `json:"pr"`
	ReplacedBy string `json:"replaced_by"`
}

type mergeResponse struct {
	PR struct {
		PullRequestID string `json:"pull_request_id"`
		Status        string `json:"status"`
	} `json:"pr"`
}

type statsResponse struct {
	ByUser map[string]int `json:"by_user"`
	ByPR   map[string]int `json:"by_pr"`
}

type bulkDeactivateResponse struct {
	TeamName           string   `json:"team_name"`
	DeactivatedUserIDs []string `json:"deactivated_user_ids"`
	Reassignments      []struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
		NewUserID     string `json:"new_user_id"`
	} `json:"reassignments"`
	TeamMembers []struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		IsActive bool   `json:"is_active"`
	} `json:"team_members"`
}

type getReviewResponse struct {
	UserID       string           `json:"user_id"`
	PullRequests []pullRequestRef `json:"pull_requests"`
}

type pullRequestRef struct {
	PullRequestID string `json:"pull_request_id"`
}

func containsPR(prs []pullRequestRef, id string) bool {
	for _, pr := range prs {
		if pr.PullRequestID == id {
			return true
		}
	}
	return false
}

// --- in-memory repositories and transactor ---

type memoryTeamRepo struct {
	mu       sync.RWMutex
	teams    map[string]domain.Team
	userRepo *memoryUserRepo
}

func newMemoryTeamRepo(userRepo *memoryUserRepo) *memoryTeamRepo {
	return &memoryTeamRepo{
		teams:    make(map[string]domain.Team),
		userRepo: userRepo,
	}
}

func (r *memoryTeamRepo) CreateTeam(_ context.Context, team domain.Team) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.teams[team.TeamName] = team
	return nil
}

func (r *memoryTeamRepo) GetTeam(_ context.Context, teamName string) (domain.Team, error) {
	r.mu.RLock()
	team, ok := r.teams[teamName]
	r.mu.RUnlock()
	if !ok {
		return domain.Team{}, domain.ErrNotFound
	}
	team.Members = r.userRepo.members(teamName)
	return team, nil
}

func (r *memoryTeamRepo) TeamExists(_ context.Context, teamName string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.teams[teamName]
	return ok, nil
}

type memoryUserRepo struct {
	mu    sync.RWMutex
	users map[string]domain.User
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{
		users: make(map[string]domain.User),
	}
}

func (r *memoryUserRepo) CreateOrUpdateUser(_ context.Context, user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[user.UserID] = user
	return nil
}

func (r *memoryUserRepo) UpdateUser(_ context.Context, user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[user.UserID]; !ok {
		return domain.ErrNotFound
	}
	r.users[user.UserID] = user
	return nil
}

func (r *memoryUserRepo) GetUser(_ context.Context, userID string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[userID]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return user, nil
}

func (r *memoryUserRepo) GetTeamMembers(_ context.Context, teamName string) ([]domain.User, error) {
	return r.members(teamName), nil
}

func (r *memoryUserRepo) DeactivateUsers(_ context.Context, teamName string, userIDs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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

func (r *memoryUserRepo) members(teamName string) []domain.User {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.User, 0)
	for _, u := range r.users {
		if u.TeamName == teamName {
			result = append(result, u)
		}
	}
	return result
}

type memoryPRRepo struct {
	mu  sync.RWMutex
	prs map[string]domain.PullRequest
}

func newMemoryPRRepo() *memoryPRRepo {
	return &memoryPRRepo{
		prs: make(map[string]domain.PullRequest),
	}
}

func (r *memoryPRRepo) CreatePR(_ context.Context, pr domain.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.prs[pr.PullRequestID]; exists {
		return fmt.Errorf("pr exists: %s", pr.PullRequestID)
	}
	r.prs[pr.PullRequestID] = pr
	return nil
}

func (r *memoryPRRepo) GetPR(_ context.Context, prID string) (domain.PullRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pr, ok := r.prs[prID]
	if !ok {
		return domain.PullRequest{}, domain.ErrNotFound
	}
	return clonePR(pr), nil
}

func (r *memoryPRRepo) UpdatePR(_ context.Context, pr domain.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.prs[pr.PullRequestID]; !ok {
		return domain.ErrNotFound
	}
	r.prs[pr.PullRequestID] = pr
	return nil
}

func (r *memoryPRRepo) AssignReviewers(_ context.Context, prID string, reviewers []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	pr, ok := r.prs[prID]
	if !ok {
		return domain.ErrNotFound
	}
	for _, reviewer := range reviewers {
		if !containsString(pr.AssignedReviewers, reviewer) {
			pr.AssignedReviewers = append(pr.AssignedReviewers, reviewer)
		}
	}
	r.prs[prID] = pr
	return nil
}

func (r *memoryPRRepo) RemoveReviewer(_ context.Context, prID string, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	pr, ok := r.prs[prID]
	if !ok {
		return domain.ErrNotFound
	}
	filtered := make([]string, 0, len(pr.AssignedReviewers))
	for _, reviewer := range pr.AssignedReviewers {
		if reviewer != userID {
			filtered = append(filtered, reviewer)
		}
	}
	pr.AssignedReviewers = filtered
	r.prs[prID] = pr
	return nil
}

func (r *memoryPRRepo) AddReviewer(_ context.Context, prID string, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	pr, ok := r.prs[prID]
	if !ok {
		return domain.ErrNotFound
	}
	if !containsString(pr.AssignedReviewers, userID) {
		pr.AssignedReviewers = append(pr.AssignedReviewers, userID)
	}
	r.prs[prID] = pr
	return nil
}

func (r *memoryPRRepo) GetPRsByReviewer(_ context.Context, userID string) ([]domain.PullRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	prs := make([]domain.PullRequest, 0)
	for _, pr := range r.prs {
		if containsString(pr.AssignedReviewers, userID) {
			copied := clonePR(pr)
			prs = append(prs, copied)
		}
	}
	return prs, nil
}

func (r *memoryPRRepo) PRExists(_ context.Context, prID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.prs[prID]
	return ok, nil
}

func (r *memoryPRRepo) GetAssignmentStatsByUser(_ context.Context) (map[string]int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stats := make(map[string]int)
	for _, pr := range r.prs {
		for _, reviewer := range pr.AssignedReviewers {
			stats[reviewer]++
		}
	}
	return stats, nil
}

func (r *memoryPRRepo) GetAssignmentStatsByPR(_ context.Context) (map[string]int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stats := make(map[string]int)
	for id, pr := range r.prs {
		stats[id] = len(pr.AssignedReviewers)
	}
	return stats, nil
}

func (r *memoryPRRepo) GetOpenPRIDsByReviewer(_ context.Context, userID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0)
	for id, pr := range r.prs {
		if pr.Status == domain.PRStatusMerged {
			continue
		}
		if containsString(pr.AssignedReviewers, userID) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func clonePR(pr domain.PullRequest) domain.PullRequest {
	copied := pr
	if pr.AssignedReviewers != nil {
		copied.AssignedReviewers = append([]string(nil), pr.AssignedReviewers...)
	}
	return copied
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

type noopTransactor struct{}

func (noopTransactor) Do(ctx context.Context, f func(ctx context.Context) error) error {
	return f(ctx)
}
