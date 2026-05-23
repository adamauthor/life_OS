package planning

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"life_os/internal/domain"
)

type fakePlanningRepository struct {
	proposal domain.ReplanProposal
}

func (r *fakePlanningRepository) SaveDailyDirection(_ context.Context, direction domain.DailyDirection) (domain.DailyDirection, error) {
	direction.ID = uuid.New()
	return direction, nil
}

func (r *fakePlanningRepository) SaveReplanProposal(_ context.Context, proposal domain.ReplanProposal) (domain.ReplanProposal, error) {
	proposal.ID = uuid.New()
	r.proposal = proposal
	return proposal, nil
}

func (r *fakePlanningRepository) GetReplanProposal(_ context.Context, _ domain.UUID) (domain.ReplanProposal, error) {
	return r.proposal, nil
}

func (r *fakePlanningRepository) GetReplanProposalForUser(_ context.Context, _ domain.UUID, _ domain.UUID) (domain.ReplanProposal, error) {
	return r.proposal, nil
}

func (r *fakePlanningRepository) UpdateReplanProposalStatus(_ context.Context, _ domain.UUID, status string, confirmedAt *time.Time) error {
	r.proposal.Status = status
	r.proposal.ConfirmedAt = confirmedAt
	return nil
}

func (r *fakePlanningRepository) UpdateReplanProposalStatusForUser(_ context.Context, _ domain.UUID, _ domain.UUID, status string, confirmedAt *time.Time) error {
	r.proposal.Status = status
	r.proposal.ConfirmedAt = confirmedAt
	return nil
}

type fakePlanningContext struct{}

func (fakePlanningContext) GetUserProfile(_ context.Context, userID domain.UUID) (domain.UserProfile, error) {
	return domain.UserProfile{UserID: userID, Goals: map[string]any{}}, nil
}

func (fakePlanningContext) ListRecentMemories(_ context.Context, _ domain.UUID, _ int) ([]domain.Memory, error) {
	return nil, nil
}

type fakePlanningReviews struct{}

func (fakePlanningReviews) ListDailyReviews(_ context.Context, _ domain.UUID, _ time.Time, _ int) ([]domain.DailyReview, error) {
	return nil, nil
}

type fakePlanningPatterns struct{}

func (fakePlanningPatterns) GetRelevantPatterns(_ context.Context, _ domain.UUID, _ string) ([]domain.BehavioralPattern, error) {
	return []domain.BehavioralPattern{{Code: "late_sleep_loop", Confidence: 0.8}}, nil
}

func (fakePlanningPatterns) ListActive(_ context.Context, _ domain.UUID) ([]domain.BehavioralPattern, error) {
	return nil, nil
}

type fakePlanningCalendar struct {
	applied   int
	available bool
}

func (c *fakePlanningCalendar) IsAvailableForUser(_ context.Context, _ domain.UUID) bool {
	return c.available
}

func (c *fakePlanningCalendar) ListDayForUser(_ context.Context, _ domain.UUID, _ time.Time) ([]domain.CalendarEvent, error) {
	return []domain.CalendarEvent{{ID: "fixed", Title: "Call", IsFixed: true}}, nil
}

func (c *fakePlanningCalendar) ApplyCalendarActionsForUser(_ context.Context, _ domain.UUID, actions []domain.ReplanCalendarAction) (string, error) {
	c.applied += len(actions)
	return "ok", nil
}

type fakePlanningAI struct{}

func (fakePlanningAI) BuildDailyDirection(_ context.Context, _ domain.DailyDirectionPromptInput) (domain.DailyDirection, error) {
	return domain.DailyDirection{}, nil
}

func (fakePlanningAI) BuildReplanProposal(_ context.Context, _ domain.ReplanPromptInput) (domain.ReplanAIResponse, error) {
	return domain.ReplanAIResponse{
		Reason: "late wake-up",
		Plan: domain.ProposedPlan{
			Date: "2026-05-23",
			Blocks: []domain.PlanBlock{
				{Type: "fixed", Title: "Call", Start: "13:00", DurationMinutes: 30, CalendarWrite: false},
				{Type: "flexible", Title: "Deep work", Start: "15:00", DurationMinutes: 60, CalendarWrite: true},
			},
		},
		CalendarActions: []domain.ReplanCalendarAction{
			{Action: "create", Title: "Deep work", Start: "2026-05-23T15:00:00+07:00", End: "2026-05-23T16:00:00+07:00", CalendarWrite: true},
		},
		AuthorityMessage: "Следующий шаг: выйти из комнаты.",
	}, nil
}

func TestBuildReplanProposalDoesNotApplyCalendarBeforeConfirm(t *testing.T) {
	repository := &fakePlanningRepository{}
	calendar := &fakePlanningCalendar{available: true}
	service := NewService(repository, fakePlanningContext{}, fakePlanningReviews{}, fakePlanningPatterns{}, calendar, fakePlanningAI{}, time.UTC)

	proposal, err := service.BuildReplanProposal(context.Background(), uuid.New(), "проспал до 11:40", time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildReplanProposal returned error: %v", err)
	}
	if proposal.Status != ProposalStatusPending {
		t.Fatalf("proposal.Status = %q, want pending", proposal.Status)
	}
	if calendar.applied != 0 {
		t.Fatalf("calendar.applied = %d, want 0 before confirm", calendar.applied)
	}
	if len(proposal.CalendarActions) != 1 {
		t.Fatalf("CalendarActions len = %d, want 1", len(proposal.CalendarActions))
	}

	if err := service.ConfirmReplan(context.Background(), proposal.ID); err != nil {
		t.Fatalf("ConfirmReplan returned error: %v", err)
	}
	if calendar.applied != 1 {
		t.Fatalf("calendar.applied = %d, want 1 after confirm", calendar.applied)
	}
	if repository.proposal.Status != ProposalStatusApplied {
		t.Fatalf("repository status = %q, want applied", repository.proposal.Status)
	}
}

func TestBuildReplanProposalStripsCalendarActionsWhenCalendarUnavailable(t *testing.T) {
	repository := &fakePlanningRepository{}
	calendar := &fakePlanningCalendar{available: false}
	service := NewService(repository, fakePlanningContext{}, fakePlanningReviews{}, fakePlanningPatterns{}, calendar, fakePlanningAI{}, time.UTC)

	proposal, err := service.BuildReplanProposal(context.Background(), uuid.New(), "проспал до 11:40", time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildReplanProposal returned error: %v", err)
	}
	if len(proposal.CalendarActions) != 0 {
		t.Fatalf("CalendarActions len = %d, want 0", len(proposal.CalendarActions))
	}
}
