package domain

import "time"

type Anchor struct {
	Type            string `json:"type"`
	Title           string `json:"title"`
	Window          string `json:"window"`
	DurationMinutes int    `json:"duration_minutes"`
	CalendarWrite   bool   `json:"calendar_write"`
}

type Priority struct {
	Title string `json:"title"`
	Why   string `json:"why"`
}

type DailyDirection struct {
	ID         UUID
	UserID     UUID
	Date       time.Time
	Text       string
	Anchors    []Anchor
	Priorities []Priority
	CreatedAt  time.Time
}

type BehavioralPattern struct {
	ID             UUID
	UserID         UUID
	Code           string
	Title          string
	Description    string
	Signals        []string
	Outcomes       []string
	CounterActions []string
	Confidence     float64
	LastSeenAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PlanBlock struct {
	Type            string `json:"type"`
	Title           string `json:"title"`
	Start           string `json:"start"`
	DurationMinutes int    `json:"duration_minutes"`
	CalendarWrite   bool   `json:"calendar_write"`
}

type ProposedPlan struct {
	Date   string      `json:"date"`
	Reason string      `json:"reason,omitempty"`
	Blocks []PlanBlock `json:"blocks"`
}

type ReplanCalendarAction struct {
	Action          string `json:"action"`
	SourceEventID   string `json:"source_event_id,omitempty"`
	Title           string `json:"title"`
	Start           string `json:"start"`
	End             string `json:"end,omitempty"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
	BlockType       string `json:"block_type,omitempty"`
	CalendarWrite   bool   `json:"calendar_write"`
}

type ReplanProposal struct {
	ID               UUID
	UserID           UUID
	Status           string
	Reason           string
	ProposedPlan     ProposedPlan
	CalendarActions  []ReplanCalendarAction
	AuthorityMessage string
	RiskDetected     string
	CreatedAt        time.Time
	ConfirmedAt      *time.Time
}

type ReplanAIResponse struct {
	Reason           string                 `json:"reason"`
	RiskDetected     string                 `json:"risk_detected"`
	Plan             ProposedPlan           `json:"plan"`
	CalendarActions  []ReplanCalendarAction `json:"calendar_actions"`
	AuthorityMessage string                 `json:"authority_message"`
}

type UserProfile struct {
	UserID          UUID
	Timezone        string
	WakeTimeTarget  string
	SleepTimeTarget string
	WorkStart       string
	WorkEnd         string
	Goals           map[string]any
	Rules           map[string]any
	PersonalityMode string
}

type HabitLog struct {
	Date  time.Time `json:"date"`
	Name  string    `json:"name"`
	Value float64   `json:"value"`
	Notes string    `json:"notes"`
}

type DailyDirectionPromptInput struct {
	UserID   UUID
	Date     time.Time
	Profile  UserProfile
	Goals    map[string]any
	Memories []Memory
	Reviews  []DailyReview
	Patterns []BehavioralPattern
	Events   []CalendarEvent
	Timezone string
	Now      time.Time
}

type ReplanPromptInput struct {
	UserID      UUID
	Date        time.Time
	Profile     UserProfile
	Memories    []Memory
	Reviews     []DailyReview
	Patterns    []BehavioralPattern
	Events      []CalendarEvent
	UserMessage string
	CurrentTime time.Time
	Timezone    string
}

type AvoidanceSignal struct {
	Detected bool
	Pattern  *BehavioralPattern
	Message  string
}

type CompanionResponseInput struct {
	Message         string
	NextStep        string
	AvoidanceSignal *AvoidanceSignal
	Patterns        []BehavioralPattern
}
