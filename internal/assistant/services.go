package assistant

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"life_os/internal/domain"
)

type UserProfileRepository interface {
	UpsertUserProfileFact(ctx context.Context, fact domain.UserProfileFact) (domain.UserProfileFact, error)
	ListUserProfileFacts(ctx context.Context, userID domain.UUID) ([]domain.UserProfileFact, error)
}

type UserProfileService struct {
	repository UserProfileRepository
}

func NewUserProfileService(repository UserProfileRepository) *UserProfileService {
	return &UserProfileService{repository: repository}
}

func (s *UserProfileService) ListFacts(ctx context.Context, userID domain.UUID) ([]domain.UserProfileFact, error) {
	if s == nil || s.repository == nil {
		return nil, nil
	}
	return s.repository.ListUserProfileFacts(ctx, userID)
}

func (s *UserProfileService) IsEmpty(ctx context.Context, userID domain.UUID) bool {
	facts, err := s.ListFacts(ctx, userID)
	return err != nil || len(facts) == 0
}

func (s *UserProfileService) UpsertFacts(ctx context.Context, userID domain.UUID, facts []ProfileFactIntent) ([]domain.UserProfileFact, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("user profile service is not configured")
	}
	saved := make([]domain.UserProfileFact, 0, len(facts))
	for _, item := range facts {
		category := normalizeKey(item.Category, "general")
		key := normalizeKey(item.Key, stableKey(item.Value))
		value := strings.TrimSpace(item.Value)
		if value == "" {
			continue
		}
		confidence := item.Confidence
		if confidence == 0 {
			confidence = 0.8
		}
		fact, err := s.repository.UpsertUserProfileFact(ctx, domain.UserProfileFact{
			UserID:     userID,
			Category:   category,
			Key:        key,
			Value:      value,
			Confidence: confidence,
			Source:     "user",
			Status:     "active",
		})
		if err != nil {
			return nil, err
		}
		saved = append(saved, fact)
	}
	return saved, nil
}

type KnowledgeRepository interface {
	CreateKnowledgeItem(ctx context.Context, item domain.KnowledgeItem) (domain.KnowledgeItem, error)
	ListKnowledgeItems(ctx context.Context, userID domain.UUID, itemType string, status string, limit int) ([]domain.KnowledgeItem, error)
	SearchKnowledgeItems(ctx context.Context, userID domain.UUID, queryEmbedding []float32, limit int) ([]domain.KnowledgeItem, error)
}

type KnowledgeService struct {
	repository KnowledgeRepository
	ai         AIClient
}

func NewKnowledgeService(repository KnowledgeRepository, ai AIClient) *KnowledgeService {
	return &KnowledgeService{repository: repository, ai: ai}
}

func (s *KnowledgeService) Save(ctx context.Context, userID domain.UUID, rawText string, parsed KnowledgeIntent) (domain.KnowledgeItem, error) {
	if s == nil || s.repository == nil {
		return domain.KnowledgeItem{}, fmt.Errorf("knowledge service is not configured")
	}
	itemType := normalizeKey(parsed.Type, "general_note")
	title := strings.TrimSpace(parsed.Title)
	if title == "" {
		title = strings.TrimSpace(parsed.Summary)
	}
	if title == "" {
		title = rawText
	}
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		summary = rawText
	}
	var dueDate *time.Time
	if strings.TrimSpace(parsed.DueDate) != "" {
		if parsedDate, err := time.Parse("2006-01-02", parsed.DueDate); err == nil {
			dueDate = &parsedDate
		}
	}
	var embedding []float32
	var err error
	if s.ai != nil {
		embedding, err = s.ai.CreateEmbedding(ctx, summary+"\n"+rawText)
		if err != nil {
			return domain.KnowledgeItem{}, err
		}
	}
	return s.repository.CreateKnowledgeItem(ctx, domain.KnowledgeItem{
		UserID:    userID,
		Type:      itemType,
		Title:     title,
		RawText:   rawText,
		Summary:   summary,
		Entities:  parsed.Entities,
		Amount:    parsed.Amount,
		Currency:  strings.TrimSpace(parsed.Currency),
		DueDate:   dueDate,
		Status:    "active",
		Tags:      parsed.Tags,
		Embedding: embedding,
	})
}

func (s *KnowledgeService) ActiveDebts(ctx context.Context, userID domain.UUID) ([]domain.KnowledgeItem, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("knowledge service is not configured")
	}
	return s.repository.ListKnowledgeItems(ctx, userID, "debt", "active", 20)
}

func (s *KnowledgeService) Search(ctx context.Context, userID domain.UUID, question string) ([]domain.KnowledgeItem, error) {
	if s == nil || s.repository == nil || s.ai == nil {
		return nil, fmt.Errorf("knowledge search is not configured")
	}
	embedding, err := s.ai.CreateEmbedding(ctx, question)
	if err != nil {
		return nil, err
	}
	return s.repository.SearchKnowledgeItems(ctx, userID, embedding, 6)
}

type AnchorRepository interface {
	UpsertAnchorPreference(ctx context.Context, pref domain.AnchorPreference) (domain.AnchorPreference, error)
	ListAnchorPreferences(ctx context.Context, userID domain.UUID) ([]domain.AnchorPreference, error)
}

type AnchorService struct {
	repository AnchorRepository
}

func NewAnchorService(repository AnchorRepository) *AnchorService {
	return &AnchorService{repository: repository}
}

func (s *AnchorService) List(ctx context.Context, userID domain.UUID) ([]domain.AnchorPreference, error) {
	if s == nil || s.repository == nil {
		return nil, nil
	}
	return s.repository.ListAnchorPreferences(ctx, userID)
}

func (s *AnchorService) ApplyFeedback(ctx context.Context, userID domain.UUID, updates []AnchorUpdateIntent) ([]domain.AnchorPreference, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("anchor service is not configured")
	}
	saved := make([]domain.AnchorPreference, 0, len(updates))
	for _, update := range updates {
		code := normalizeKey(update.AnchorCode, stableKey(update.Title))
		title := strings.TrimSpace(update.Title)
		if title == "" {
			title = strings.ReplaceAll(code, "_", " ")
		}
		status := strings.TrimSpace(update.Status)
		if status == "" {
			status = "neutral"
		}
		pref, err := s.repository.UpsertAnchorPreference(ctx, domain.AnchorPreference{
			UserID:          userID,
			AnchorCode:      code,
			Title:           title,
			PreferenceScore: update.PreferenceScore,
			Status:          status,
			Reason:          strings.TrimSpace(update.Reason),
		})
		if err != nil {
			return nil, err
		}
		saved = append(saved, pref)
	}
	return saved, nil
}

var nonKeyPattern = regexp.MustCompile(`[^a-zа-я0-9]+`)

func normalizeKey(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonKeyPattern.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return fallback
	}
	return value
}

func stableKey(value string) string {
	value = normalizeKey(value, "fact")
	parts := strings.Split(value, "_")
	if len(parts) > 4 {
		parts = parts[:4]
	}
	return strings.Join(parts, "_")
}
