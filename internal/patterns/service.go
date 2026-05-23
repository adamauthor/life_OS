package patterns

import (
	"context"
	"regexp"
	"strings"
	"time"

	"life_os/internal/domain"
)

type Repository interface {
	UpsertBehavioralPatterns(ctx context.Context, patterns []domain.BehavioralPattern) error
	ListBehavioralPatterns(ctx context.Context, userID domain.UUID, limit int, minConfidence float64) ([]domain.BehavioralPattern, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) ExtractPatternsFromReview(ctx context.Context, review domain.DailyReview) ([]domain.BehavioralPattern, error) {
	now := s.now()
	patterns := make([]domain.BehavioralPattern, 0, len(review.Patterns)+4)
	seen := map[string]struct{}{}

	for _, detected := range review.Patterns {
		code := normalizeCode(detected.Code)
		if code == "" {
			code = normalizeCode(detected.Title)
		}
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		confidence := detected.Confidence
		if confidence <= 0 {
			confidence = 0.55
		}
		patterns = append(patterns, domain.BehavioralPattern{
			UserID:         review.UserID,
			Code:           code,
			Title:          fallback(detected.Title, code),
			Description:    fallback(detected.Description, detected.Title),
			Signals:        detected.Signals,
			Outcomes:       detected.Outcomes,
			CounterActions: detected.CounterActions,
			Confidence:     clampConfidence(confidence),
			LastSeenAt:     &now,
		})
	}

	text := strings.ToLower(strings.Join([]string{
		review.RawText,
		review.Summary,
		strings.Join(review.Failures, " "),
		strings.Join(review.Harmed, " "),
		strings.Join(review.Helped, " "),
	}, " "))

	for _, pattern := range heuristicPatterns(review.UserID, text, now) {
		if _, ok := seen[pattern.Code]; ok {
			continue
		}
		seen[pattern.Code] = struct{}{}
		patterns = append(patterns, pattern)
	}

	return patterns, nil
}

func (s *Service) UpdatePatternConfidence(ctx context.Context, userID domain.UUID, patterns []domain.BehavioralPattern) error {
	for i := range patterns {
		patterns[i].UserID = userID
		patterns[i].Confidence = clampConfidence(patterns[i].Confidence)
		if patterns[i].LastSeenAt == nil {
			now := s.now()
			patterns[i].LastSeenAt = &now
		}
	}
	return s.repository.UpsertBehavioralPatterns(ctx, patterns)
}

func (s *Service) GetRelevantPatterns(ctx context.Context, userID domain.UUID, contextText string) ([]domain.BehavioralPattern, error) {
	patterns, err := s.repository.ListBehavioralPatterns(ctx, userID, 10, 0.3)
	if err != nil {
		return nil, err
	}
	contextText = strings.ToLower(contextText)
	if strings.TrimSpace(contextText) == "" {
		return patterns, nil
	}
	relevant := make([]domain.BehavioralPattern, 0, len(patterns))
	for _, pattern := range patterns {
		haystack := strings.ToLower(pattern.Code + " " + pattern.Title + " " + pattern.Description + " " + strings.Join(pattern.Signals, " "))
		if strings.Contains(contextText, pattern.Code) || overlap(contextText, haystack) {
			relevant = append(relevant, pattern)
		}
	}
	if len(relevant) == 0 {
		return patterns, nil
	}
	return relevant, nil
}

func (s *Service) ListActive(ctx context.Context, userID domain.UUID) ([]domain.BehavioralPattern, error) {
	return s.repository.ListBehavioralPatterns(ctx, userID, 10, 0.3)
}

func heuristicPatterns(userID domain.UUID, text string, now time.Time) []domain.BehavioralPattern {
	candidates := []domain.BehavioralPattern{}
	add := func(code, title, description string, signals, outcomes, counterActions []string, confidence float64) {
		candidates = append(candidates, domain.BehavioralPattern{
			UserID:         userID,
			Code:           code,
			Title:          title,
			Description:    description,
			Signals:        signals,
			Outcomes:       outcomes,
			CounterActions: counterActions,
			Confidence:     confidence,
			LastSeenAt:     &now,
		})
	}

	if containsAny(text, "лег в 4", "до 4", "после 02", "после 2", "поздно лег", "не выспался", "late sleep") {
		add(
			"late_sleep_loop",
			"Late sleep loop",
			"Поздний сон сдвигает следующий день и снижает управляемость.",
			[]string{"late bedtime", "low sleep", "late wake-up"},
			[]string{"late wake-up", "lost morning", "lower energy"},
			[]string{"set hard shutdown window", "move body outside", "one deep work block only"},
			0.65,
		)
	}
	if containsAny(text, "телефон", "скролл", "doomscroll", "reels", "shorts", "youtube", "instagram", "залип") {
		add(
			"doomscrolling_loop",
			"Doomscrolling loop",
			"Телефон используется как избегание и съедает восстановление.",
			[]string{"phone in bed", "scrolling", "avoidance"},
			[]string{"lost time", "late sleep", "worse mood"},
			[]string{"put phone away", "stand up", "leave room for 20 minutes"},
			0.62,
		)
	}
	if containsAny(text, "комната", "кровать", "не вышел", "изол", "номер") && containsAny(text, "телефон", "лежал", "слил", "хуже", "плохо") {
		add(
			"isolation_after_work",
			"Isolation after work",
			"Комната, кровать и телефон после нагрузки запускают избегание.",
			[]string{"staying inside", "bed", "phone"},
			[]string{"late sleep", "low mood", "missed movement"},
			[]string{"shower", "leave room", "walk 20 minutes"},
			0.64,
		)
	}
	if containsAny(text, "прогул", "движение", "зал", "тренировка", "плавал", "walk", "movement") && containsAny(text, "помог", "лучше", "сброс", "вернул") {
		add(
			"movement_resets_day",
			"Movement resets day",
			"Движение возвращает контроль над днем.",
			[]string{"walk", "training", "movement"},
			[]string{"better mood", "higher control"},
			[]string{"start with 20-30 minutes outside"},
			0.68,
		)
	}
	if containsAny(text, "новое место", "исслед", "exploration", "вышел", "море", "кафе") && containsAny(text, "помог", "лучше", "энерг", "настроение") {
		add(
			"exploration_improves_mood",
			"Exploration improves mood",
			"Смена среды и исследование места улучшают состояние.",
			[]string{"new place", "outside", "exploration"},
			[]string{"better mood", "more energy"},
			[]string{"schedule a low-friction outside anchor"},
			0.66,
		)
	}
	return candidates
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func overlap(contextText, haystack string) bool {
	for _, field := range strings.Fields(contextText) {
		if len([]rune(field)) < 5 {
			continue
		}
		if strings.Contains(haystack, field) {
			return true
		}
	}
	return false
}

var nonCodeCharacter = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func normalizeCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonCodeCharacter.ReplaceAllString(value, "_")
	return strings.Trim(value, "_")
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func clampConfidence(value float64) float64 {
	if value < 0.01 {
		return 0.01
	}
	if value > 0.99 {
		return 0.99
	}
	return value
}
