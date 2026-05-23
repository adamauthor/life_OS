package planning

import (
	"context"
	"fmt"
	"strings"
	"time"

	"life_os/internal/domain"
)

const (
	ProposalStatusPending   = "pending"
	ProposalStatusConfirmed = "confirmed"
	ProposalStatusApplied   = "applied"
	ProposalStatusCancelled = "cancelled"
	ProposalStatusFailed    = "failed"
)

type Repository interface {
	SaveDailyDirection(ctx context.Context, direction domain.DailyDirection) (domain.DailyDirection, error)
	SaveReplanProposal(ctx context.Context, proposal domain.ReplanProposal) (domain.ReplanProposal, error)
	GetReplanProposal(ctx context.Context, proposalID domain.UUID) (domain.ReplanProposal, error)
	GetReplanProposalForUser(ctx context.Context, userID domain.UUID, proposalID domain.UUID) (domain.ReplanProposal, error)
	UpdateReplanProposalStatus(ctx context.Context, proposalID domain.UUID, status string, confirmedAt *time.Time) error
	UpdateReplanProposalStatusForUser(ctx context.Context, userID domain.UUID, proposalID domain.UUID, status string, confirmedAt *time.Time) error
}

type ContextRepository interface {
	GetUserProfile(ctx context.Context, userID domain.UUID) (domain.UserProfile, error)
	ListRecentMemories(ctx context.Context, userID domain.UUID, limit int) ([]domain.Memory, error)
}

type ReviewRepository interface {
	ListDailyReviews(ctx context.Context, userID domain.UUID, since time.Time, limit int) ([]domain.DailyReview, error)
}

type PatternProvider interface {
	GetRelevantPatterns(ctx context.Context, userID domain.UUID, context string) ([]domain.BehavioralPattern, error)
	ListActive(ctx context.Context, userID domain.UUID) ([]domain.BehavioralPattern, error)
}

type Calendar interface {
	IsAvailableForUser(ctx context.Context, userID domain.UUID) bool
	ListDayForUser(ctx context.Context, userID domain.UUID, day time.Time) ([]domain.CalendarEvent, error)
	ApplyCalendarActionsForUser(ctx context.Context, userID domain.UUID, actions []domain.ReplanCalendarAction) (string, error)
}

type AIClient interface {
	BuildDailyDirection(ctx context.Context, input domain.DailyDirectionPromptInput) (domain.DailyDirection, error)
	BuildReplanProposal(ctx context.Context, input domain.ReplanPromptInput) (domain.ReplanAIResponse, error)
}

type Service struct {
	repository Repository
	contexts   ContextRepository
	reviews    ReviewRepository
	patterns   PatternProvider
	calendar   Calendar
	ai         AIClient
	timezone   *time.Location
}

func NewService(repository Repository, contexts ContextRepository, reviews ReviewRepository, patterns PatternProvider, calendar Calendar, ai AIClient, timezone *time.Location) *Service {
	if timezone == nil {
		timezone = time.UTC
	}
	return &Service{
		repository: repository,
		contexts:   contexts,
		reviews:    reviews,
		patterns:   patterns,
		calendar:   calendar,
		ai:         ai,
		timezone:   timezone,
	}
}

func (s *Service) BuildDailyDirection(ctx context.Context, userID domain.UUID, date time.Time) (*domain.DailyDirection, error) {
	loc := date.Location()
	if loc == nil {
		loc = s.timezone
	}
	date = startOfDay(date.In(loc))
	contextData := s.loadContext(ctx, userID, date, "today")

	input := domain.DailyDirectionPromptInput{
		UserID:   userID,
		Date:     date,
		Profile:  contextData.profile,
		Goals:    contextData.profile.Goals,
		Memories: contextData.memories,
		Reviews:  contextData.reviews,
		Patterns: contextData.patterns,
		Events:   contextData.events,
		Timezone: loc.String(),
		Now:      time.Now().In(loc),
	}

	var direction domain.DailyDirection
	var err error
	if s.ai != nil {
		direction, err = s.ai.BuildDailyDirection(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("build daily direction: %w", err)
		}
	} else {
		direction = fallbackDailyDirection(userID, date)
	}
	direction.UserID = userID
	direction.Date = date
	direction.Anchors = limitAnchors(direction.Anchors)
	direction.Priorities = limitPriorities(direction.Priorities)
	if strings.TrimSpace(direction.Text) == "" {
		direction.Text = directionText(direction)
	}

	saved, err := s.repository.SaveDailyDirection(ctx, direction)
	if err != nil {
		return nil, fmt.Errorf("save daily direction: %w", err)
	}
	return &saved, nil
}

func (s *Service) BuildReplanProposal(ctx context.Context, userID domain.UUID, inputText string, date time.Time) (*domain.ReplanProposal, error) {
	loc := date.Location()
	if loc == nil {
		loc = s.timezone
	}
	date = startOfDay(date.In(loc))
	contextData := s.loadContext(ctx, userID, date, inputText)

	input := domain.ReplanPromptInput{
		UserID:      userID,
		Date:        date,
		Profile:     contextData.profile,
		Memories:    contextData.memories,
		Reviews:     contextData.reviews,
		Patterns:    contextData.patterns,
		Events:      contextData.events,
		UserMessage: strings.TrimSpace(inputText),
		CurrentTime: time.Now().In(loc),
		Timezone:    loc.String(),
	}

	var response domain.ReplanAIResponse
	var err error
	if s.ai != nil {
		response, err = s.ai.BuildReplanProposal(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("build replan proposal: %w", err)
		}
	} else {
		response = fallbackReplanResponse(date, inputText)
	}
	if response.Plan.Date == "" {
		response.Plan.Date = date.Format("2006-01-02")
	}
	if response.Plan.Reason == "" {
		response.Plan.Reason = response.Reason
	}
	if response.Reason == "" {
		response.Reason = response.Plan.Reason
	}

	proposal := domain.ReplanProposal{
		UserID:           userID,
		Status:           ProposalStatusPending,
		Reason:           response.Reason,
		ProposedPlan:     response.Plan,
		CalendarActions:  s.normalizeCalendarActionsForUser(ctx, userID, response.CalendarActions),
		AuthorityMessage: response.AuthorityMessage,
		RiskDetected:     response.RiskDetected,
	}
	saved, err := s.repository.SaveReplanProposal(ctx, proposal)
	if err != nil {
		return nil, fmt.Errorf("save replan proposal: %w", err)
	}
	return &saved, nil
}

func (s *Service) ConfirmReplan(ctx context.Context, proposalID domain.UUID) error {
	return s.confirmReplan(ctx, domain.UUID{}, proposalID)
}

func (s *Service) ConfirmReplanForUser(ctx context.Context, userID domain.UUID, proposalID domain.UUID) error {
	return s.confirmReplan(ctx, userID, proposalID)
}

func (s *Service) confirmReplan(ctx context.Context, userID domain.UUID, proposalID domain.UUID) error {
	proposal, err := s.getReplanProposal(ctx, userID, proposalID)
	if err != nil {
		return fmt.Errorf("get replan proposal: %w", err)
	}
	if proposal.Status != ProposalStatusPending {
		return fmt.Errorf("replan proposal is not pending")
	}

	now := time.Now().In(s.timezone)
	if err := s.updateReplanProposalStatus(ctx, userID, proposalID, ProposalStatusConfirmed, &now); err != nil {
		return err
	}
	if s.calendar != nil {
		if _, err := s.calendar.ApplyCalendarActionsForUser(ctx, proposal.UserID, proposal.CalendarActions); err != nil {
			_ = s.updateReplanProposalStatus(ctx, userID, proposalID, ProposalStatusFailed, &now)
			return fmt.Errorf("apply calendar actions: %w", err)
		}
	}
	if err := s.updateReplanProposalStatus(ctx, userID, proposalID, ProposalStatusApplied, &now); err != nil {
		return err
	}
	return nil
}

func (s *Service) CancelReplan(ctx context.Context, proposalID domain.UUID) error {
	return s.repository.UpdateReplanProposalStatus(ctx, proposalID, ProposalStatusCancelled, nil)
}

func (s *Service) CancelReplanForUser(ctx context.Context, userID domain.UUID, proposalID domain.UUID) error {
	return s.repository.UpdateReplanProposalStatusForUser(ctx, userID, proposalID, ProposalStatusCancelled, nil)
}

func (s *Service) getReplanProposal(ctx context.Context, userID domain.UUID, proposalID domain.UUID) (domain.ReplanProposal, error) {
	if userID == (domain.UUID{}) {
		return s.repository.GetReplanProposal(ctx, proposalID)
	}
	return s.repository.GetReplanProposalForUser(ctx, userID, proposalID)
}

func (s *Service) updateReplanProposalStatus(ctx context.Context, userID domain.UUID, proposalID domain.UUID, status string, confirmedAt *time.Time) error {
	if userID == (domain.UUID{}) {
		return s.repository.UpdateReplanProposalStatus(ctx, proposalID, status, confirmedAt)
	}
	return s.repository.UpdateReplanProposalStatusForUser(ctx, userID, proposalID, status, confirmedAt)
}

type planningContext struct {
	profile  domain.UserProfile
	memories []domain.Memory
	reviews  []domain.DailyReview
	patterns []domain.BehavioralPattern
	events   []domain.CalendarEvent
}

func (s *Service) loadContext(ctx context.Context, userID domain.UUID, date time.Time, patternContext string) planningContext {
	data := planningContext{
		profile: domain.UserProfile{
			UserID:          userID,
			Timezone:        s.timezone.String(),
			Goals:           map[string]any{},
			Rules:           map[string]any{},
			PersonalityMode: "authority_companion",
		},
	}
	if s.contexts != nil {
		if profile, err := s.contexts.GetUserProfile(ctx, userID); err == nil {
			data.profile = profile
		}
		if memories, err := s.contexts.ListRecentMemories(ctx, userID, 12); err == nil {
			data.memories = memories
		}
	}
	if s.reviews != nil {
		if reviews, err := s.reviews.ListDailyReviews(ctx, userID, date.AddDate(0, 0, -7), 7); err == nil {
			data.reviews = reviews
		}
	}
	if s.patterns != nil {
		if patterns, err := s.patterns.GetRelevantPatterns(ctx, userID, patternContext); err == nil {
			data.patterns = patterns
		}
	}
	if s.calendar != nil {
		if events, err := s.calendar.ListDayForUser(ctx, userID, date); err == nil {
			data.events = events
		}
	}
	return data
}

func (s *Service) normalizeCalendarActionsForUser(ctx context.Context, userID domain.UUID, actions []domain.ReplanCalendarAction) []domain.ReplanCalendarAction {
	if s.calendar == nil || !s.calendar.IsAvailableForUser(ctx, userID) {
		return nil
	}
	normalized := make([]domain.ReplanCalendarAction, 0, len(actions))
	for _, action := range actions {
		action.Action = strings.ToLower(strings.TrimSpace(action.Action))
		if action.Action == "" {
			action.Action = "create"
		}
		if !action.CalendarWrite {
			continue
		}
		if action.Action != "create" && action.Action != "update" {
			continue
		}
		normalized = append(normalized, action)
	}
	return normalized
}

func limitAnchors(anchors []domain.Anchor) []domain.Anchor {
	if len(anchors) > 5 {
		anchors = anchors[:5]
	}
	if len(anchors) < 3 {
		defaults := fallbackDailyDirection(domain.UUID{}, time.Now()).Anchors
		for _, anchor := range defaults {
			if len(anchors) >= 3 {
				break
			}
			anchors = append(anchors, anchor)
		}
	}
	return anchors
}

func limitPriorities(priorities []domain.Priority) []domain.Priority {
	if len(priorities) > 3 {
		priorities = priorities[:3]
	}
	if len(priorities) == 0 {
		priorities = []domain.Priority{{Title: "1 deep work блок", Why: "Главный рычаг дня."}}
	}
	return priorities
}

func directionText(direction domain.DailyDirection) string {
	lines := []string{}
	for _, anchor := range direction.Anchors {
		lines = append(lines, anchor.Title)
	}
	return strings.Join(lines, "\n")
}

func fallbackDailyDirection(userID domain.UUID, date time.Time) domain.DailyDirection {
	return domain.DailyDirection{
		UserID: userID,
		Date:   date,
		Text:   "День держится на простых якорях: выйти из комнаты, движение, один deep work блок и нормальный shutdown.",
		Anchors: []domain.Anchor{
			{Type: "anchor", Title: "Выйти из комнаты", Window: "первая свободная дневная пауза", DurationMinutes: 20, CalendarWrite: false},
			{Type: "anchor", Title: "Движение 30+ минут", Window: "день или ранний вечер", DurationMinutes: 30, CalendarWrite: false},
			{Type: "flexible", Title: "1 deep work блок", Window: "ближайшее окно без fixed events", DurationMinutes: 60, CalendarWrite: false},
			{Type: "anchor", Title: "Не лечь спать после 02:00", Window: "вечер", DurationMinutes: 30, CalendarWrite: false},
		},
		Priorities: []domain.Priority{
			{Title: "1 deep work блок", Why: "Не размазывать день."},
		},
	}
}

func fallbackReplanResponse(date time.Time, reason string) domain.ReplanAIResponse {
	return domain.ReplanAIResponse{
		Reason:       fallback(reason, "manual replan"),
		RiskDetected: "режим может снова съехать без внешнего якоря",
		Plan: domain.ProposedPlan{
			Date:   date.Format("2006-01-02"),
			Reason: fallback(reason, "manual replan"),
			Blocks: []domain.PlanBlock{
				{Type: "anchor", Title: "Выйти из комнаты", Start: "сейчас + 20 минут", DurationMinutes: 30, CalendarWrite: false},
				{Type: "flexible", Title: "1 deep work block", Start: "первое свободное окно", DurationMinutes: 60, CalendarWrite: true},
				{Type: "recovery", Title: "Прогулка без телефона", Start: "вечер", DurationMinutes: 45, CalendarWrite: false},
			},
		},
		AuthorityMessage: "Ты не возвращаешь потерянное утро. Ты забираешь следующий рабочий блок.",
	}
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}
