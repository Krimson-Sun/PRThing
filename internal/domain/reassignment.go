package domain

// Reassignment describes reviewer replacement details.
type Reassignment struct {
	PullRequestID string
	OldUserID     string
	NewUserID     string
}
