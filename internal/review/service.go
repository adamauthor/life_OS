package review

import (
	"context"
	"fmt"
	"strings"
	"time"

	"life_os/internal/domain"
)

type Repository interface {
	SaveDailyReview(ctx context.Context, review domain.DailyReview) (domain.DailyReview, error)
	ListDailyReviews(ctx context.Context, userID domain.UUID, since time.Time, limit int) ([]domain.DailyReview, error)
}

type ContextRepository interface {
	ListRecentMemories(ctx context.Context, userID domain.UUID, limit int) ([]domain.Memory, error)
	ListHabitLogs(ctx context.Context, userID domain.UUID, since time.Time) ([]domain.HabitLog, error)
}

type MemoryWriter interface {
	CreateMemory(ctx context.Context, memory domain.Memory) (domain.Memory, error)
}

type AIClient interface {
	AnalyzeDailyReview(ctx context.Context, rawText string, recentMemories []domain.Memory, previousPatterns []domain.BehavioralPattern) (domain.DailyReview, error)
	BuildWeeklyReview(ctx context.Context, input domain.WeeklyReviewInput) (string, error)
	CreateEmbedding(ctx context.Context, text string) ([]float32, error)
}

type PatternService interface {
	ExtractPatternsFromReview(ctx context.Context, review domain.DailyReview) ([]domain.BehavioralPattern, error)
	UpdatePatternConfidence(ctx context.Context, userID domain.UUID, patterns []domain.BehavioralPattern) error
	ListActive(ctx context.Context, userID domain.UUID) ([]domain.BehavioralPattern, error)
}

type CalendarReader interface {
	ListDay(ctx context.Context, day time.Time) ([]domain.CalendarEvent, error)
}

type Service struct {
	repository Repository
	contexts   ContextRepository
	memories   MemoryWriter
	ai         AIClient
	patterns   PatternService
	calendar   CalendarReader
	timezone   *time.Location
}

func NewService(repository Repository, contexts ContextRepository, memories MemoryWriter, ai AIClient, patterns PatternService, calendar CalendarReader, timezone *time.Location) *Service {
	if timezone == nil {
		timezone = time.UTC
	}
	return &Service{
		repository: repository,
		contexts:   contexts,
		memories:   memories,
		ai:         ai,
		patterns:   patterns,
		calendar:   calendar,
		timezone:   timezone,
	}
}

func (s *Service) StartDailyReview(ctx context.Context, userID domain.UUID) error {
	return nil
}

func (s *Service) SaveDailyReview(ctx context.Context, userID domain.UUID, rawText string) (*domain.DailyReview, error) {
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return nil, fmt.Errorf("review text is required")
	}

	recentMemories := []domain.Memory{}
	if s.contexts != nil {
		memories, err := s.contexts.ListRecentMemories(ctx, userID, 10)
		if err == nil {
			recentMemories = memories
		}
	}
	previousPatterns := []domain.BehavioralPattern{}
	if s.patterns != nil {
		patterns, err := s.patterns.ListActive(ctx, userID)
		if err == nil {
			previousPatterns = patterns
		}
	}

	var parsed domain.DailyReview
	var err error
	if s.ai != nil {
		parsed, err = s.ai.AnalyzeDailyReview(ctx, rawText, recentMemories, previousPatterns)
		if err != nil {
			return nil, fmt.Errorf("analyze daily review: %w", err)
		}
	} else {
		parsed = domain.DailyReview{Summary: rawText}
	}

	now := time.Now().In(s.timezone)
	parsed.UserID = userID
	parsed.RawText = rawText
	parsed.ReviewDate = startOfDay(now)
	parsed.Date = parsed.ReviewDate

	saved, err := s.repository.SaveDailyReview(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("save daily review: %w", err)
	}

	if s.patterns != nil {
		detected, err := s.patterns.ExtractPatternsFromReview(ctx, saved)
		if err != nil {
			return nil, fmt.Errorf("extract patterns from review: %w", err)
		}
		if len(detected) > 0 {
			if err := s.patterns.UpdatePatternConfidence(ctx, userID, detected); err != nil {
				return nil, fmt.Errorf("update behavioral patterns: %w", err)
			}
		}
	}

	if s.memories != nil && s.ai != nil {
		if err := s.saveReviewMemory(ctx, saved); err != nil {
			return nil, err
		}
	}

	return &saved, nil
}

func (s *Service) BuildWeeklyReview(ctx context.Context, userID domain.UUID, weekStart time.Time) (string, error) {
	weekStart = startOfDay(weekStart.In(s.timezone))
	weekEnd := weekStart.AddDate(0, 0, 7)

	reviews := []domain.DailyReview{}
	memories := []domain.Memory{}
	habitLogs := []domain.HabitLog{}
	patterns := []domain.BehavioralPattern{}
	events := []domain.CalendarEvent{}

	if s.repository != nil {
		value, err := s.repository.ListDailyReviews(ctx, userID, weekStart, 10)
		if err == nil {
			reviews = value
		}
	}
	if s.contexts != nil {
		value, err := s.contexts.ListRecentMemories(ctx, userID, 30)
		if err == nil {
			memories = value
		}
		valueLogs, err := s.contexts.ListHabitLogs(ctx, userID, weekStart)
		if err == nil {
			habitLogs = valueLogs
		}
	}
	if s.patterns != nil {
		value, err := s.patterns.ListActive(ctx, userID)
		if err == nil {
			patterns = value
		}
	}
	if s.calendar != nil {
		for day := weekStart; day.Before(weekEnd); day = day.AddDate(0, 0, 1) {
			dayEvents, err := s.calendar.ListDay(ctx, day)
			if err == nil {
				events = append(events, dayEvents...)
			}
		}
	}

	input := domain.WeeklyReviewInput{
		UserID:    userID,
		WeekStart: weekStart,
		WeekEnd:   weekEnd,
		Memories:  memories,
		Reviews:   reviews,
		Events:    events,
		HabitLogs: habitLogs,
		Patterns:  patterns,
	}
	if s.ai == nil {
		return fallbackWeeklyReview(input), nil
	}
	text, err := s.ai.BuildWeeklyReview(ctx, input)
	if err != nil {
		return "", fmt.Errorf("build weekly review: %w", err)
	}
	return text, nil
}

func (s *Service) saveReviewMemory(ctx context.Context, review domain.DailyReview) error {
	embeddingText := strings.TrimSpace(review.Summary)
	if embeddingText == "" {
		embeddingText = review.RawText
	}
	embedding, err := s.ai.CreateEmbedding(ctx, embeddingText)
	if err != nil {
		return fmt.Errorf("create review embedding: %w", err)
	}
	memory, err := domain.NewMemory(domain.NewMemoryInput{
		UserID:    review.UserID,
		Type:      domain.MemoryTypeReflection,
		RawText:   review.RawText,
		Summary:   embeddingText,
		Tags:      []string{"review", "daily"},
		Source:    "daily_review",
		Embedding: embedding,
		Metadata: map[string]any{
			"review_id":   review.ID.String(),
			"user_uuid":   review.UserID.String(),
			"review_date": review.ReviewDate.Format("2006-01-02"),
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

func fallbackWeeklyReview(input domain.WeeklyReviewInput) string {
	lines := []string{
		"Что работало",
		"- Недостаточно AI-контекста для полного анализа. Смотри последние review и patterns.",
		"",
		"Что ломало режим",
		"- Проверь поздний сон, изоляцию и телефон, если они повторялись.",
		"",
		"Главный паттерн недели",
		"- " + firstPattern(input.Patterns),
		"",
		"Главная проблема",
		"- Нет устойчивого недельного вывода без review.",
		"",
		"Фокус следующей недели",
		"- Один deep work блок в день и жесткий вечерний shutdown.",
	}
	return strings.Join(lines, "\n")
}

func firstPattern(patterns []domain.BehavioralPattern) string {
	if len(patterns) == 0 {
		return "pattern пока не накоплен"
	}
	return patterns[0].Code
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}
