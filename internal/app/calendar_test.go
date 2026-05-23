package app

import (
	"context"
	"testing"
	"time"

	"life_os/internal/domain"
)

type fakeCalendarRepository struct {
	nextID  int64
	actions map[int64]domain.CalendarAction
}

func newFakeCalendarRepository() *fakeCalendarRepository {
	return &fakeCalendarRepository{nextID: 1, actions: make(map[int64]domain.CalendarAction)}
}

func (r *fakeCalendarRepository) CreateCalendarAction(_ context.Context, action domain.CalendarAction) (domain.CalendarAction, error) {
	action.ID = r.nextID
	r.nextID++
	r.actions[action.ID] = action
	return action, nil
}

func (r *fakeCalendarRepository) GetCalendarAction(_ context.Context, _ domain.UUID, id int64) (domain.CalendarAction, error) {
	return r.actions[id], nil
}

func (r *fakeCalendarRepository) UpdateCalendarActionStatus(_ context.Context, _ domain.UUID, id int64, status domain.CalendarActionStatus) error {
	action := r.actions[id]
	action.Status = status
	r.actions[id] = action
	return nil
}

type fakeCalendarClient struct {
	created int
	updated []string
}

func (c *fakeCalendarClient) CreateEvent(_ context.Context, _ CreateCalendarEventInput) (string, error) {
	c.created++
	return "created", nil
}

func (c *fakeCalendarClient) UpdateEvent(_ context.Context, eventID string, _ UpdateCalendarEventInput) error {
	c.updated = append(c.updated, eventID)
	return nil
}

func (c *fakeCalendarClient) ListEvents(_ context.Context, _ time.Time) ([]CalendarEvent, error) {
	return nil, nil
}

func TestConfirmReplanSkipsFixedAndAppliesChanges(t *testing.T) {
	repository := newFakeCalendarRepository()
	calendar := &fakeCalendarClient{}
	service := NewCalendarService(repository, calendar)

	userID := domain.UserIDFromTelegram(123)
	action, err := service.ProposeReplan(context.Background(), userID, ReplanProposal{
		Summary: "test replan",
		Events: []ReplanProposalItem{
			{SourceEventID: "fixed", Title: "Fixed", Start: "2026-05-23T10:00:00+07:00", End: "2026-05-23T11:00:00+07:00", IsFixed: true, Action: "keep"},
			{SourceEventID: "move", Title: "Move", Start: "2026-05-23T12:00:00+07:00", End: "2026-05-23T13:00:00+07:00", Action: "update"},
			{Title: "New", Start: "2026-05-23T14:00:00+07:00", End: "2026-05-23T15:00:00+07:00", Action: "create"},
		},
	})
	if err != nil {
		t.Fatalf("ProposeReplan returned error: %v", err)
	}

	result, err := service.ConfirmAction(context.Background(), userID, action.ID)
	if err != nil {
		t.Fatalf("ConfirmAction returned error: %v", err)
	}
	if result != "applied 2 calendar changes" {
		t.Fatalf("result = %q", result)
	}
	if len(calendar.updated) != 1 || calendar.updated[0] != "move" {
		t.Fatalf("updated = %#v, want only movable event", calendar.updated)
	}
	if calendar.created != 1 {
		t.Fatalf("created = %d, want 1", calendar.created)
	}
	if repository.actions[action.ID].Status != domain.CalendarActionStatusApplied {
		t.Fatalf("status = %q, want applied", repository.actions[action.ID].Status)
	}
}

func TestApplyCalendarActionsValidatesBeforeWriting(t *testing.T) {
	repository := newFakeCalendarRepository()
	calendar := &fakeCalendarClient{}
	service := NewCalendarService(repository, calendar)

	_, err := service.ApplyCalendarActions(context.Background(), []domain.ReplanCalendarAction{
		{
			Action:          "create",
			Title:           "Deep work",
			Start:           "2026-05-23T14:00:00+07:00",
			DurationMinutes: 60,
			CalendarWrite:   true,
		},
		{
			Action:          "update",
			Title:           "Move meeting",
			Start:           "2026-05-23T16:00:00+07:00",
			DurationMinutes: 60,
			CalendarWrite:   true,
		},
	})
	if err == nil {
		t.Fatal("ApplyCalendarActions returned nil error for invalid update")
	}
	if calendar.created != 0 {
		t.Fatalf("created = %d, want 0 because validation must happen before writes", calendar.created)
	}
	if len(calendar.updated) != 0 {
		t.Fatalf("updated = %#v, want no writes", calendar.updated)
	}
}

func TestRestrictedCalendarBlocksOtherUsers(t *testing.T) {
	repository := newFakeCalendarRepository()
	calendar := &fakeCalendarClient{}
	service := NewCalendarService(repository, calendar)
	service.RestrictToUser(domain.UserIDFromTelegram(1))

	otherUser := domain.UserIDFromTelegram(2)
	_, err := service.ProposeEvent(context.Background(), otherUser, domain.ParsedIntent{
		Title:           "Deep work",
		Datetime:        "2026-05-23T15:00:00+07:00",
		DurationMinutes: 60,
	})
	if err == nil {
		t.Fatal("ProposeEvent returned nil error for non-owner")
	}
	if len(repository.actions) != 0 {
		t.Fatalf("actions len = %d, want 0", len(repository.actions))
	}
	if service.IsAvailableForUser(context.Background(), otherUser) {
		t.Fatal("calendar should not be available for non-owner")
	}
}
