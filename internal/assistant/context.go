package assistant

import (
	"context"
	"time"

	"life_os/internal/domain"
)

type AssistantInput struct {
	UserID     domain.UUID
	TelegramID int64
	ChatID     int64
	Text       string
	Source     string
	Now        time.Time
	Timezone   string
}

type AssistantButton struct {
	Text string
	Data string
	URL  string
}

type AssistantResponse struct {
	Text    string
	Buttons []AssistantButton
}

type IntentInput struct {
	Text     string
	Now      time.Time
	Timezone string
	Facts    []domain.UserProfileFact
	Anchors  []domain.AnchorPreference
}

type ParsedIntent struct {
	Intent                AssistantIntent       `json:"intent"`
	Confidence            float64               `json:"confidence"`
	Language              string                `json:"language"`
	RequiresClarification bool                  `json:"requires_clarification"`
	ClarificationQuestion string                `json:"clarification_question"`
	Calendar              *CalendarIntent       `json:"calendar"`
	Knowledge             *KnowledgeIntent      `json:"knowledge"`
	ProfileUpdate         *ProfileUpdateIntent  `json:"profile_update"`
	AnchorFeedback        *AnchorFeedbackIntent `json:"anchor_feedback"`
	Query                 *QueryIntent          `json:"query"`
}

type CalendarIntent struct {
	Title                string   `json:"title"`
	StartTime            string   `json:"start_time"`
	EndTime              string   `json:"end_time"`
	DurationMinutes      int      `json:"duration_minutes"`
	Range                string   `json:"range"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	Tags                 []string `json:"tags"`
}

type KnowledgeIntent struct {
	Type     string         `json:"type"`
	Title    string         `json:"title"`
	Summary  string         `json:"summary"`
	Entities map[string]any `json:"entities"`
	Amount   *float64       `json:"amount"`
	Currency string         `json:"currency"`
	DueDate  string         `json:"due_date"`
	Tags     []string       `json:"tags"`
}

type ProfileUpdateIntent struct {
	Facts []ProfileFactIntent `json:"facts"`
}

type ProfileFactIntent struct {
	Category   string  `json:"category"`
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
}

type AnchorFeedbackIntent struct {
	Updates []AnchorUpdateIntent `json:"updates"`
}

type AnchorUpdateIntent struct {
	AnchorCode      string  `json:"anchor_code"`
	Title           string  `json:"title"`
	PreferenceScore float64 `json:"preference_score"`
	Status          string  `json:"status"`
	Reason          string  `json:"reason"`
}

type QueryIntent struct {
	Text  string `json:"text"`
	Type  string `json:"type"`
	Range string `json:"range"`
}

type AIClient interface {
	ParseAssistantIntent(ctx context.Context, input IntentInput) (ParsedIntent, error)
	CreateEmbedding(ctx context.Context, text string) ([]float32, error)
}

type CalendarService interface {
	ProposeEvent(ctx context.Context, userID domain.UUID, parsed domain.ParsedIntent) (domain.CalendarAction, error)
	ListDayForUser(ctx context.Context, userID domain.UUID, day time.Time) ([]domain.CalendarEvent, error)
}

type PlanningService interface {
	BuildDailyDirection(ctx context.Context, userID domain.UUID, date time.Time) (*domain.DailyDirection, error)
	BuildReplanProposal(ctx context.Context, userID domain.UUID, input string, date time.Time) (*domain.ReplanProposal, error)
}

type MemoryService interface {
	AnswerQuestion(ctx context.Context, userID domain.UUID, question string) (string, error)
}
