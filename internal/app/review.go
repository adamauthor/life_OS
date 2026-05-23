package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"life_os/internal/domain"
)

type ReviewRepository interface {
	SaveDailyReview(ctx context.Context, review domain.DailyReview) (domain.DailyReview, error)
}

type ReviewService struct {
	repository ReviewRepository
	memories   MemoryRepository
	ai         AIClient
}

func NewReviewService(repository ReviewRepository, memories MemoryRepository, ai AIClient) *ReviewService {
	return &ReviewService{repository: repository, memories: memories, ai: ai}
}

func (s *ReviewService) SaveDailyReview(ctx context.Context, rawText string, date time.Time) (domain.DailyReview, error) {
	if strings.TrimSpace(rawText) == "" {
		return domain.DailyReview{}, fmt.Errorf("review text is required")
	}
	review, err := s.ai.SummarizeDailyReview(ctx, rawText)
	if err != nil {
		return domain.DailyReview{}, fmt.Errorf("summarize daily review: %w", err)
	}
	review.Date = date
	review.RawText = rawText

	saved, err := s.repository.SaveDailyReview(ctx, review)
	if err != nil {
		return domain.DailyReview{}, fmt.Errorf("save daily review: %w", err)
	}
	if s.memories != nil {
		if err := s.saveReviewMemory(ctx, saved); err != nil {
			return domain.DailyReview{}, err
		}
	}
	return saved, nil
}

func (s *ReviewService) saveReviewMemory(ctx context.Context, review domain.DailyReview) error {
	embedding, err := s.ai.CreateEmbedding(ctx, review.Summary)
	if err != nil {
		return fmt.Errorf("create review embedding: %w", err)
	}
	memory, err := domain.NewMemory(domain.NewMemoryInput{
		Type:      domain.MemoryTypeReflection,
		RawText:   review.RawText,
		Summary:   review.Summary,
		Tags:      []string{"review", "daily"},
		Source:    "daily_review",
		Embedding: embedding,
		Metadata: map[string]any{
			"review_id": review.ID,
			"date":      review.Date.Format("2006-01-02"),
			"mood":      review.Mood,
			"energy":    review.Energy,
		},
	})
	if err != nil {
		return fmt.Errorf("build review memory: %w", err)
	}
	if _, err := s.memories.CreateMemory(ctx, memory); err != nil {
		return fmt.Errorf("save review memory: %w", err)
	}
	return nil
}
