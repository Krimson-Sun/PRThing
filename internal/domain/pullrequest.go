package domain

import "time"

type PRStatus string

const (
	PRStatusOpen   PRStatus = "OPEN"
	PRStatusMerged PRStatus = "MERGED"
)

type PullRequest struct {
	PullRequestID     string
	PullRequestName   string
	AuthorID          string
	Status            PRStatus
	AssignedReviewers []string
	CreatedAt         time.Time
	MergedAt          *time.Time
}

func NewPullRequest(prID, prName, authorID string) PullRequest {
	return PullRequest{
		PullRequestID:     prID,
		PullRequestName:   prName,
		AuthorID:          authorID,
		Status:            PRStatusOpen,
		AssignedReviewers: make([]string, 0),
		CreatedAt:         time.Now(),
		MergedAt:          nil,
	}
}

func (pr *PullRequest) IsMerged() bool {
	return pr.Status == PRStatusMerged
}

func (pr *PullRequest) CanReassign() bool {
	return !pr.IsMerged()
}

func (pr *PullRequest) Merge() {
	if pr.IsMerged() {
		return
	}
	pr.Status = PRStatusMerged
	now := time.Now()
	pr.MergedAt = &now
}

func (pr *PullRequest) IsReviewerAssigned(userID string) bool {
	for _, rid := range pr.AssignedReviewers {
		if rid == userID {
			return true
		}
	}
	return false
}

func (pr *PullRequest) ReplaceReviewer(oldUserID, newUserID string) error {
	if pr.IsMerged() {
		return ErrPRMerged
	}
	if !pr.IsReviewerAssigned(oldUserID) {
		return ErrNotAssigned
	}

	for i, rid := range pr.AssignedReviewers {
		if rid == oldUserID {
			pr.AssignedReviewers[i] = newUserID
			return nil
		}
	}
	return ErrNotAssigned
}

func (pr *PullRequest) AddReviewer(userID string) {
	if !pr.IsReviewerAssigned(userID) {
		pr.AssignedReviewers = append(pr.AssignedReviewers, userID)
	}
}

func (pr *PullRequest) SetReviewers(reviewers []string) {
	pr.AssignedReviewers = reviewers
}
