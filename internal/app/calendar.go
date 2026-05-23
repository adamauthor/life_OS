package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"life_os/internal/domain"
)

type CalendarRepository interface {
	CreateCalendarAction(ctx context.Context, action domain.CalendarAction) (domain.CalendarAction, error)
	GetCalendarAction(ctx context.Context, userID domain.UUID, id int64) (domain.CalendarAction, error)
	UpdateCalendarActionStatus(ctx context.Context, userID domain.UUID, id int64, status domain.CalendarActionStatus) error
}

type CalendarClient interface {
	CreateEvent(ctx context.Context, input CreateCalendarEventInput) (string, error)
	UpdateEvent(ctx context.Context, eventID string, input UpdateCalendarEventInput) error
	ListEvents(ctx context.Context, day time.Time) ([]CalendarEvent, error)
}

type CalendarService struct {
	repository CalendarRepository
	calendar   CalendarClient
}

func NewCalendarService(repository CalendarRepository, calendar CalendarClient) *CalendarService {
	return &CalendarService{repository: repository, calendar: calendar}
}

func (s *CalendarService) ProposeEvent(ctx context.Context, userID domain.UUID, parsed domain.ParsedIntent) (domain.CalendarAction, error) {
	start, err := parsed.EventTime()
	if err != nil {
		return domain.CalendarAction{}, fmt.Errorf("parse event time: %w", err)
	}
	duration := parsed.DurationMinutes
	if duration <= 0 {
		duration = 60
	}

	return s.repository.CreateCalendarAction(ctx, domain.CalendarAction{
		UserID:     userID,
		ActionType: "create_event",
		Status:     domain.CalendarActionStatusPending,
		ProposedPayload: map[string]any{
			"title":            parsed.Title,
			"datetime":         start.Format(time.RFC3339),
			"duration_minutes": duration,
			"tags":             parsed.Tags,
		},
	})
}

func (s *CalendarService) ConfirmAction(ctx context.Context, userID domain.UUID, id int64) (string, error) {
	action, err := s.repository.GetCalendarAction(ctx, userID, id)
	if err != nil {
		return "", fmt.Errorf("get calendar action: %w", err)
	}
	if action.Status != domain.CalendarActionStatusPending {
		return "", fmt.Errorf("calendar action is not pending")
	}
	if s.calendar == nil {
		return "", fmt.Errorf("calendar adapter is not configured")
	}

	if err := s.repository.UpdateCalendarActionStatus(ctx, userID, id, domain.CalendarActionStatusConfirmed); err != nil {
		return "", fmt.Errorf("confirm calendar action: %w", err)
	}

	result, err := s.applyAction(ctx, action)
	if err != nil {
		_ = s.repository.UpdateCalendarActionStatus(ctx, userID, id, domain.CalendarActionStatusFailed)
		return "", err
	}
	if err := s.repository.UpdateCalendarActionStatus(ctx, userID, id, domain.CalendarActionStatusApplied); err != nil {
		return "", fmt.Errorf("mark calendar action applied: %w", err)
	}
	return result, nil
}

func (s *CalendarService) CancelAction(ctx context.Context, userID domain.UUID, id int64) error {
	return s.repository.UpdateCalendarActionStatus(ctx, userID, id, domain.CalendarActionStatusCancelled)
}

func (s *CalendarService) ListDay(ctx context.Context, day time.Time) ([]CalendarEvent, error) {
	if s.calendar == nil {
		return nil, fmt.Errorf("calendar adapter is not configured")
	}
	return s.calendar.ListEvents(ctx, day)
}

func (s *CalendarService) ApplyCalendarActions(ctx context.Context, actions []domain.ReplanCalendarAction) (string, error) {
	writable := 0
	for _, action := range actions {
		if action.CalendarWrite {
			writable++
		}
	}
	if writable == 0 {
		return "no calendar changes", nil
	}
	if s.calendar == nil {
		return "", fmt.Errorf("calendar adapter is not configured")
	}

	applied := 0
	for _, action := range actions {
		if !action.CalendarWrite {
			continue
		}
		actionType := strings.ToLower(strings.TrimSpace(action.Action))
		if actionType == "" {
			actionType = "create"
		}
		start, err := time.Parse(time.RFC3339, action.Start)
		if err != nil {
			return "", fmt.Errorf("parse calendar action start: %w", err)
		}
		end, err := replanActionEnd(action, start)
		if err != nil {
			return "", err
		}
		switch actionType {
		case "create", "create_event":
			duration := int(end.Sub(start).Minutes())
			if duration <= 0 {
				duration = action.DurationMinutes
			}
			if duration <= 0 {
				duration = 60
			}
			if _, err := s.calendar.CreateEvent(ctx, CreateCalendarEventInput{
				Title:           action.Title,
				Start:           start,
				DurationMinutes: duration,
			}); err != nil {
				return "", fmt.Errorf("create planned calendar event: %w", err)
			}
			applied++
		case "update", "update_event":
			if strings.TrimSpace(action.SourceEventID) == "" {
				return "", fmt.Errorf("update calendar action requires source_event_id")
			}
			if err := s.calendar.UpdateEvent(ctx, action.SourceEventID, UpdateCalendarEventInput{
				Title: action.Title,
				Start: start,
				End:   end,
			}); err != nil {
				return "", fmt.Errorf("update planned calendar event: %w", err)
			}
			applied++
		default:
			return "", fmt.Errorf("unsupported replan calendar action %q", action.Action)
		}
	}
	return fmt.Sprintf("applied %d calendar changes", applied), nil
}

func (s *CalendarService) ProposeReplan(ctx context.Context, userID domain.UUID, proposal ReplanProposal) (domain.CalendarAction, error) {
	payload := map[string]any{
		"summary": proposal.Summary,
		"events":  proposal.Events,
		"notes":   proposal.Notes,
	}
	return s.repository.CreateCalendarAction(ctx, domain.CalendarAction{
		UserID:          userID,
		ActionType:      "replan_day",
		Status:          domain.CalendarActionStatusPending,
		ProposedPayload: payload,
	})
}

type CreateCalendarEventInput struct {
	Title           string
	Start           time.Time
	DurationMinutes int
}

type UpdateCalendarEventInput struct {
	Title string
	Start time.Time
	End   time.Time
}

func (s *CalendarService) applyAction(ctx context.Context, action domain.CalendarAction) (string, error) {
	switch action.ActionType {
	case "create_event":
		input, err := calendarEventInput(action.ProposedPayload)
		if err != nil {
			return "", err
		}
		eventID, err := s.calendar.CreateEvent(ctx, input)
		if err != nil {
			return "", fmt.Errorf("create calendar event: %w", err)
		}
		return eventID, nil
	case "replan_day":
		proposal, err := replanProposalFromPayload(action.ProposedPayload)
		if err != nil {
			return "", err
		}
		applied := 0
		for _, item := range proposal.Events {
			if item.IsFixed || item.Action == "keep" {
				continue
			}
			start, err := time.Parse(time.RFC3339, item.Start)
			if err != nil {
				return "", fmt.Errorf("parse replan item start: %w", err)
			}
			end, err := time.Parse(time.RFC3339, item.End)
			if err != nil {
				return "", fmt.Errorf("parse replan item end: %w", err)
			}
			if item.Action == "update" && item.SourceEventID != "" {
				if err := s.calendar.UpdateEvent(ctx, item.SourceEventID, UpdateCalendarEventInput{Title: item.Title, Start: start, End: end}); err != nil {
					return "", fmt.Errorf("update calendar event: %w", err)
				}
				applied++
				continue
			}
			if item.Action == "create" {
				duration := int(end.Sub(start).Minutes())
				if duration <= 0 {
					duration = 60
				}
				if _, err := s.calendar.CreateEvent(ctx, CreateCalendarEventInput{Title: item.Title, Start: start, DurationMinutes: duration}); err != nil {
					return "", fmt.Errorf("create replan event: %w", err)
				}
				applied++
			}
		}
		return fmt.Sprintf("applied %d calendar changes", applied), nil
	default:
		return "", fmt.Errorf("unsupported calendar action type %q", action.ActionType)
	}
}

func calendarEventInput(payload map[string]any) (CreateCalendarEventInput, error) {
	title, _ := payload["title"].(string)
	datetime, _ := payload["datetime"].(string)
	if title == "" || datetime == "" {
		return CreateCalendarEventInput{}, fmt.Errorf("calendar action payload is invalid")
	}
	start, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		return CreateCalendarEventInput{}, fmt.Errorf("parse calendar action datetime: %w", err)
	}
	duration, ok := payload["duration_minutes"].(float64)
	if !ok || duration <= 0 {
		duration = 60
	}
	return CreateCalendarEventInput{
		Title:           title,
		Start:           start,
		DurationMinutes: int(duration),
	}, nil
}

func replanProposalFromPayload(payload map[string]any) (ReplanProposal, error) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return ReplanProposal{}, fmt.Errorf("marshal replan payload: %w", err)
	}
	var proposal ReplanProposal
	if err := json.Unmarshal(bytes, &proposal); err != nil {
		return ReplanProposal{}, fmt.Errorf("unmarshal replan payload: %w", err)
	}
	return proposal, nil
}

func replanActionEnd(action domain.ReplanCalendarAction, start time.Time) (time.Time, error) {
	if strings.TrimSpace(action.End) != "" {
		end, err := time.Parse(time.RFC3339, action.End)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse calendar action end: %w", err)
		}
		return end, nil
	}
	duration := action.DurationMinutes
	if duration <= 0 {
		duration = 60
	}
	return start.Add(time.Duration(duration) * time.Minute), nil
}
