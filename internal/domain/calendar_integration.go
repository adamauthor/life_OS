package domain

import "time"

type GoogleCalendarConnection struct {
	ID          UUID
	UserID      UUID
	CalendarID  string
	TokenJSON   string
	ConnectedAt time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OAuthState struct {
	State     string
	UserID    UUID
	ChatID    int64
	Provider  string
	ExpiresAt time.Time
	CreatedAt time.Time
}
