package companion

import (
	"context"
	"fmt"
	"strings"

	"life_os/internal/domain"
)

type PatternProvider interface {
	GetRelevantPatterns(ctx context.Context, userID domain.UUID, context string) ([]domain.BehavioralPattern, error)
}

type Service struct {
	patterns PatternProvider
}

func NewService(patterns PatternProvider) *Service {
	return &Service{patterns: patterns}
}

func (s *Service) FormatAuthorityResponse(ctx context.Context, input domain.CompanionResponseInput) (string, error) {
	lines := []string{}
	if input.AvoidanceSignal != nil && input.AvoidanceSignal.Detected {
		if input.AvoidanceSignal.Pattern != nil {
			lines = append(lines, fmt.Sprintf("Ты снова уходишь в сценарий %q.", input.AvoidanceSignal.Pattern.Code))
		} else if strings.TrimSpace(input.AvoidanceSignal.Message) != "" {
			lines = append(lines, input.AvoidanceSignal.Message)
		}
	}
	if strings.TrimSpace(input.Message) != "" {
		lines = append(lines, strings.TrimSpace(input.Message))
	}
	if strings.TrimSpace(input.NextStep) != "" {
		lines = append(lines, "Следующий шаг: "+strings.TrimSpace(input.NextStep))
	}
	if len(lines) == 0 {
		lines = append(lines, "Следующий шаг: выбери одно действие и начни сейчас.")
	}
	return strings.Join(lines, "\n"), nil
}

func (s *Service) DetectAvoidanceContext(ctx context.Context, userID domain.UUID, input string) (*domain.AvoidanceSignal, error) {
	if s.patterns == nil {
		return &domain.AvoidanceSignal{Detected: false}, nil
	}
	relevant, err := s.patterns.GetRelevantPatterns(ctx, userID, input)
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(input)
	for _, pattern := range relevant {
		if pattern.Code == "isolation_after_work" && containsAny(lower, "комната", "кровать", "телефон", "номер", "лежу") {
			patternCopy := pattern
			return &domain.AvoidanceSignal{Detected: true, Pattern: &patternCopy}, nil
		}
		if pattern.Code == "late_sleep_loop" && containsAny(lower, "лег в", "поздно", "проспал", "не выспался") {
			patternCopy := pattern
			return &domain.AvoidanceSignal{Detected: true, Pattern: &patternCopy}, nil
		}
		if pattern.Code == "doomscrolling_loop" && containsAny(lower, "телефон", "скролл", "залип", "youtube", "reels") {
			patternCopy := pattern
			return &domain.AvoidanceSignal{Detected: true, Pattern: &patternCopy}, nil
		}
	}
	return &domain.AvoidanceSignal{Detected: false}, nil
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
