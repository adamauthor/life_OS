package app

import (
	"context"
	"io"

	"life_os/internal/domain"
)

type AIClient interface {
	ParseIntent(ctx context.Context, text string, nowRFC3339 string, timezone string) (domain.ParsedIntent, error)
	CreateEmbedding(ctx context.Context, text string) ([]float32, error)
	Transcribe(ctx context.Context, filename string, audio io.Reader) (string, error)
	AnswerWithMemories(ctx context.Context, question string, memories []domain.Memory) (string, error)
	SummarizeDailyReview(ctx context.Context, rawText string) (domain.DailyReview, error)
	ReplanDay(ctx context.Context, message string, calendarEvents []CalendarEvent) (ReplanProposal, error)
}

type CalendarEvent struct {
	ID          string
	Title       string
	Start       string
	End         string
	IsFixed     bool
	Description string
}

type ReplanProposal struct {
	Summary string               `json:"summary"`
	Events  []ReplanProposalItem `json:"events"`
	Notes   []string             `json:"notes"`
}

type ReplanProposalItem struct {
	SourceEventID string `json:"source_event_id"`
	Title         string `json:"title"`
	Start         string `json:"start"`
	End           string `json:"end"`
	IsFixed       bool   `json:"is_fixed"`
	Action        string `json:"action"`
}
