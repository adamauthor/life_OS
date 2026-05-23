package domain

import "time"

type CalendarEvent struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Start       string `json:"start"`
	End         string `json:"end"`
	IsFixed     bool   `json:"is_fixed"`
	Description string `json:"description"`
}

type CalendarActionStatus string

const (
	CalendarActionStatusPending   CalendarActionStatus = "pending"
	CalendarActionStatusConfirmed CalendarActionStatus = "confirmed"
	CalendarActionStatusApplied   CalendarActionStatus = "applied"
	CalendarActionStatusCancelled CalendarActionStatus = "cancelled"
	CalendarActionStatusFailed    CalendarActionStatus = "failed"
)

type CalendarAction struct {
	ID              int64
	UserID          UUID
	ActionType      string
	Status          CalendarActionStatus
	ProposedPayload map[string]any
	ConfirmedAt     *time.Time
	CreatedAt       time.Time
}
