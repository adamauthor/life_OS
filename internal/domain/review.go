package domain

import "time"

type DailyReview struct {
	ID            UUID
	UserID        UUID
	ReviewDate    time.Time
	RawText       string
	Summary       string
	Wins          []string
	Failures      []string
	Helped        []string
	Harmed        []string
	TomorrowFocus []string
	Patterns      []DetectedPattern
	CreatedAt     time.Time

	// Legacy MVP fields kept so older app-level code can keep compiling while
	// the adaptive review flow uses ReviewDate and structured sections.
	Date   time.Time
	Mood   string
	Energy int
}

type DetectedPattern struct {
	Code           string   `json:"code"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Signals        []string `json:"signals"`
	Outcomes       []string `json:"outcomes"`
	CounterActions []string `json:"counter_actions"`
	Confidence     float64  `json:"confidence"`
}

type WeeklyReviewInput struct {
	UserID    UUID
	WeekStart time.Time
	WeekEnd   time.Time
	Memories  []Memory
	Reviews   []DailyReview
	Events    []CalendarEvent
	HabitLogs []HabitLog
	Patterns  []BehavioralPattern
}
