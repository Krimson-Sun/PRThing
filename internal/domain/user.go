package domain

import "time"

// User represents a team member
type User struct {
	UserID    string
	Username  string
	TeamName  string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewUser creates a new user
func NewUser(userID, username, teamName string, isActive bool) User {
	now := time.Now()
	return User{
		UserID:    userID,
		Username:  username,
		TeamName:  teamName,
		IsActive:  isActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Activate activates the user
func (u *User) Activate() {
	u.IsActive = true
	u.UpdatedAt = time.Now()
}

// Deactivate deactivates the user
func (u *User) Deactivate() {
	u.IsActive = false
	u.UpdatedAt = time.Now()
}

// CanBeReviewer checks if user can be assigned as reviewer
func (u *User) CanBeReviewer() bool {
	return u.IsActive
}

// SetIsActive sets the user's active status
func (u *User) SetIsActive(isActive bool) {
	if isActive {
		u.Activate()
	} else {
		u.Deactivate()
	}
}
