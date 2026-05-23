package domain

import "time"

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
	ActionType      string
	Status          CalendarActionStatus
	ProposedPayload map[string]any
	ConfirmedAt     *time.Time
	CreatedAt       time.Time
}
