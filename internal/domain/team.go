package domain

import "time"

// Team represents a team of users
type Team struct {
	TeamName  string
	Members   []User
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewTeam creates a new team
func NewTeam(teamName string, members []User) Team {
	now := time.Now()
	return Team{
		TeamName:  teamName,
		Members:   members,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// GetActiveMembers returns only active members
func (t *Team) GetActiveMembers() []User {
	active := make([]User, 0, len(t.Members))
	for _, m := range t.Members {
		if m.IsActive {
			active = append(active, m)
		}
	}
	return active
}

// GetActiveMembersExcluding returns active members excluding specified user
func (t *Team) GetActiveMembersExcluding(userID string) []User {
	active := make([]User, 0, len(t.Members))
	for _, m := range t.Members {
		if m.IsActive && m.UserID != userID {
			active = append(active, m)
		}
	}
	return active
}

// HasMember checks if user is in team
func (t *Team) HasMember(userID string) bool {
	for _, m := range t.Members {
		if m.UserID == userID {
			return true
		}
	}
	return false
}
