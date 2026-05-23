package domain

import "time"

type Intent string

const (
	IntentCaptureMemory       Intent = "capture_memory"
	IntentCreateTask          Intent = "create_task"
	IntentCreateCalendarEvent Intent = "create_calendar_event"
	IntentReplanDay           Intent = "replan_day"
	IntentAskMemory           Intent = "ask_memory"
	IntentDailyReview         Intent = "daily_review"
	IntentWeeklyReview        Intent = "weekly_review"
	IntentHabitLog            Intent = "habit_log"
	IntentUnknown             Intent = "unknown"
)

func (i Intent) Valid() bool {
	switch i {
	case IntentCaptureMemory, IntentCreateTask, IntentCreateCalendarEvent, IntentReplanDay, IntentAskMemory, IntentDailyReview, IntentWeeklyReview, IntentHabitLog, IntentUnknown:
		return true
	default:
		return false
	}
}

type ParsedIntent struct {
	Intent               Intent     `json:"intent"`
	Type                 MemoryType `json:"type"`
	Title                string     `json:"title"`
	RawText              string     `json:"raw_text"`
	Summary              string     `json:"summary"`
	Datetime             string     `json:"datetime"`
	DurationMinutes      int        `json:"duration_minutes"`
	Tags                 []string   `json:"tags"`
	Confidence           float64    `json:"confidence"`
	RequiresConfirmation bool       `json:"requires_confirmation"`
}

func (p ParsedIntent) EventTime() (time.Time, error) {
	return time.Parse(time.RFC3339, p.Datetime)
}
