package app

import (
	"context"
	"fmt"
	"strings"

	"life_os/internal/domain"
)

type MemoryRepository interface {
	CreateMemory(ctx context.Context, memory domain.Memory) (domain.Memory, error)
	SearchMemories(ctx context.Context, queryEmbedding []float32, limit int) ([]domain.Memory, error)
}

type MemoryService struct {
	repository MemoryRepository
	ai         AIClient
}

func NewMemoryService(repository MemoryRepository, ai AIClient) *MemoryService {
	return &MemoryService{repository: repository, ai: ai}
}

func (s *MemoryService) CaptureTelegramText(ctx context.Context, input CaptureTelegramTextInput) (domain.Memory, error) {
	parsed, err := s.ai.ParseIntent(ctx, input.Text, input.NowRFC3339, input.Timezone)
	if err != nil {
		return domain.Memory{}, fmt.Errorf("parse intent: %w", err)
	}
	return s.CaptureParsedTelegramText(ctx, input, parsed)
}

func (s *MemoryService) CaptureParsedTelegramText(ctx context.Context, input CaptureTelegramTextInput, parsed domain.ParsedIntent) (domain.Memory, error) {
	memoryType := parsed.Type
	if memoryType == "" {
		memoryType = domain.MemoryTypeNote
	}
	summary := parsed.Summary
	if strings.TrimSpace(summary) == "" {
		summary = input.Text
	}
	embedding, err := s.ai.CreateEmbedding(ctx, summary+"\n"+input.Text)
	if err != nil {
		return domain.Memory{}, fmt.Errorf("create embedding: %w", err)
	}

	memory, err := domain.NewMemory(domain.NewMemoryInput{
		Type:      memoryType,
		RawText:   input.Text,
		Summary:   summary,
		Tags:      parsed.Tags,
		Source:    "telegram",
		Embedding: embedding,
		Metadata: map[string]any{
			"intent":      parsed.Intent,
			"confidence":  parsed.Confidence,
			"chat_id":     input.ChatID,
			"message_id":  input.MessageID,
			"user_id":     input.UserID,
			"username":    input.Username,
			"telegram_at": input.TelegramAt,
		},
	})
	if err != nil {
		return domain.Memory{}, fmt.Errorf("build memory: %w", err)
	}

	saved, err := s.repository.CreateMemory(ctx, memory)
	if err != nil {
		return domain.Memory{}, fmt.Errorf("create memory: %w", err)
	}

	return saved, nil
}

func (s *MemoryService) AnswerQuestion(ctx context.Context, question string) (string, error) {
	embedding, err := s.ai.CreateEmbedding(ctx, question)
	if err != nil {
		return "", fmt.Errorf("create query embedding: %w", err)
	}

	memories, err := s.repository.SearchMemories(ctx, embedding, 6)
	if err != nil {
		return "", fmt.Errorf("search memories: %w", err)
	}

	answer, err := s.ai.AnswerWithMemories(ctx, question, memories)
	if err != nil {
		return "", fmt.Errorf("answer with memories: %w", err)
	}
	return answer, nil
}

type CaptureTelegramTextInput struct {
	Text       string
	ChatID     int64
	MessageID  int
	UserID     int64
	Username   string
	TelegramAt int
	Timezone   string
	NowRFC3339 string
}
