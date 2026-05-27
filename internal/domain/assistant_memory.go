package domain

import "time"

type UserProfileFact struct {
	ID         UUID
	UserID     UUID
	Category   string
	Key        string
	Value      string
	Confidence float64
	Source     string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type KnowledgeItem struct {
	ID        UUID
	UserID    UUID
	Type      string
	Title     string
	RawText   string
	Summary   string
	Entities  map[string]any
	Amount    *float64
	Currency  string
	DueDate   *time.Time
	Status    string
	Tags      []string
	Embedding []float32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AnchorPreference struct {
	ID              UUID
	UserID          UUID
	AnchorCode      string
	Title           string
	PreferenceScore float64
	Status          string
	Reason          string
	LastFeedbackAt  *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
