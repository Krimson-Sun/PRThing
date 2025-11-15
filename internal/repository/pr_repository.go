package repository

import (
	"context"
	"fmt"

	"pr-service/internal/db"
	"pr-service/internal/domain"

	"github.com/georgysavva/scany/v2/pgxscan"
)

type prRepository struct {
	BaseRepository
}

func NewPRRepository(cm db.EngineFactory) PRRepository {
	return &prRepository{
		BaseRepository: NewBaseRepository(cm),
	}
}

func (r *prRepository) CreatePR(ctx context.Context, pr domain.PullRequest) error {
	query := `
		INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, created_at, merged_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.Engine(ctx).Exec(ctx, query,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, pr.CreatedAt, pr.MergedAt)
	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}
	return nil
}

func (r *prRepository) GetPR(ctx context.Context, prID string) (domain.PullRequest, error) {
	// Get PR details
	prQuery := `
		SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
		FROM pull_requests
		WHERE pull_request_id = $1
	`
	var pr domain.PullRequest
	err := pgxscan.Get(ctx, r.Engine(ctx), &pr, prQuery, prID)
	if err != nil {
		if pgxscan.NotFound(err) {
			return domain.PullRequest{}, domain.ErrNotFound
		}
		return domain.PullRequest{}, fmt.Errorf("failed to get PR: %w", err)
	}

	// Get reviewers
	reviewersQuery := `
		SELECT user_id
		FROM pr_reviewers
		WHERE pull_request_id = $1
		ORDER BY assigned_at
	`
	var reviewers []string
	err = pgxscan.Select(ctx, r.Engine(ctx), &reviewers, reviewersQuery, prID)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("failed to get PR reviewers: %w", err)
	}

	pr.AssignedReviewers = reviewers
	return pr, nil
}

func (r *prRepository) UpdatePR(ctx context.Context, pr domain.PullRequest) error {
	query := `
		UPDATE pull_requests
		SET pull_request_name = $2, author_id = $3, status = $4, merged_at = $5
		WHERE pull_request_id = $1
	`
	tag, err := r.Engine(ctx).Exec(ctx, query,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, pr.MergedAt)
	if err != nil {
		return fmt.Errorf("failed to update PR: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *prRepository) AssignReviewers(ctx context.Context, prID string, reviewers []string) error {
	if len(reviewers) == 0 {
		return nil
	}

	query := `
		INSERT INTO pr_reviewers (pull_request_id, user_id, assigned_at)
		VALUES ($1, $2, NOW())
	`
	for _, userID := range reviewers {
		_, err := r.Engine(ctx).Exec(ctx, query, prID, userID)
		if err != nil {
			return fmt.Errorf("failed to assign reviewer %s: %w", userID, err)
		}
	}
	return nil
}

func (r *prRepository) RemoveReviewer(ctx context.Context, prID string, userID string) error {
	query := `
		DELETE FROM pr_reviewers
		WHERE pull_request_id = $1 AND user_id = $2
	`
	tag, err := r.Engine(ctx).Exec(ctx, query, prID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove reviewer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *prRepository) AddReviewer(ctx context.Context, prID string, userID string) error {
	query := `
		INSERT INTO pr_reviewers (pull_request_id, user_id, assigned_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (pull_request_id, user_id) DO NOTHING
	`
	_, err := r.Engine(ctx).Exec(ctx, query, prID, userID)
	if err != nil {
		return fmt.Errorf("failed to add reviewer: %w", err)
	}
	return nil
}

func (r *prRepository) GetPRsByReviewer(ctx context.Context, userID string) ([]domain.PullRequest, error) {
	query := `
		SELECT DISTINCT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status, pr.created_at, pr.merged_at
		FROM pull_requests pr
		INNER JOIN pr_reviewers rev ON pr.pull_request_id = rev.pull_request_id
		WHERE rev.user_id = $1
		ORDER BY pr.created_at DESC
	`
	var prs []domain.PullRequest
	err := pgxscan.Select(ctx, r.Engine(ctx), &prs, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PRs by reviewer: %w", err)
	}

	for i := range prs {
		prs[i].AssignedReviewers = []string{}
	}

	return prs, nil
}

// PRExists checks if a PR exists
func (r *prRepository) PRExists(ctx context.Context, prID string) (bool, error) {
	query := `
		SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)
	`
	var exists bool
	err := pgxscan.Get(ctx, r.Engine(ctx), &exists, query, prID)
	if err != nil {
		return false, fmt.Errorf("failed to check PR existence: %w", err)
	}
	return exists, nil
}

// GetAssignmentStatsByUser returns assignment count per user
func (r *prRepository) GetAssignmentStatsByUser(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT user_id, COUNT(*) as assignment_count
		FROM pr_reviewers
		GROUP BY user_id
		ORDER BY assignment_count DESC
	`
	rows, err := r.Engine(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get assignment stats by user: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var userID string
		var count int
		if err := rows.Scan(&userID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		stats[userID] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return stats, nil
}

// GetAssignmentStatsByPR returns assignment count per PR
func (r *prRepository) GetAssignmentStatsByPR(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT pull_request_id, COUNT(*) as reviewer_count
		FROM pr_reviewers
		GROUP BY pull_request_id
		ORDER BY reviewer_count DESC
	`
	rows, err := r.Engine(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get assignment stats by PR: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var prID string
		var count int
		if err := rows.Scan(&prID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		stats[prID] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return stats, nil
}
