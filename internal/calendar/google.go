package calendar

import (
	"context"
	"fmt"
	"strings"
	"time"

	gcalendar "google.golang.org/api/calendar/v3"

	"life_os/internal/app"
)

type GoogleClient struct {
	calendarID string
	service    *gcalendar.Service
}

func (c *GoogleClient) CreateEvent(ctx context.Context, input app.CreateCalendarEventInput) (string, error) {
	end := input.Start.Add(time.Duration(input.DurationMinutes) * time.Minute)
	event := &gcalendar.Event{
		Summary: input.Title,
		Start: &gcalendar.EventDateTime{
			DateTime: input.Start.Format(time.RFC3339),
		},
		End: &gcalendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
		},
	}
	created, err := c.service.Events.Insert(c.calendarID, event).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("insert google calendar event: %w", err)
	}
	return created.Id, nil
}

func (c *GoogleClient) UpdateEvent(ctx context.Context, eventID string, input app.UpdateCalendarEventInput) error {
	event := &gcalendar.Event{
		Summary: input.Title,
		Start: &gcalendar.EventDateTime{
			DateTime: input.Start.Format(time.RFC3339),
		},
		End: &gcalendar.EventDateTime{
			DateTime: input.End.Format(time.RFC3339),
		},
	}
	if _, err := c.service.Events.Patch(c.calendarID, eventID, event).Context(ctx).Do(); err != nil {
		return fmt.Errorf("patch google calendar event: %w", err)
	}
	return nil
}

func (c *GoogleClient) ListEvents(ctx context.Context, day time.Time) ([]app.CalendarEvent, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	end := start.Add(24 * time.Hour)
	result, err := c.service.Events.List(c.calendarID).
		Context(ctx).
		TimeMin(start.Format(time.RFC3339)).
		TimeMax(end.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("list google calendar events: %w", err)
	}

	events := make([]app.CalendarEvent, 0, len(result.Items))
	for _, item := range result.Items {
		events = append(events, app.CalendarEvent{
			ID:          item.Id,
			Title:       item.Summary,
			Start:       calendarDateTime(item.Start),
			End:         calendarDateTime(item.End),
			IsFixed:     isFixedEvent(item),
			Description: item.Description,
		})
	}
	return events, nil
}

func isFixedEvent(event *gcalendar.Event) bool {
	if event == nil {
		return false
	}
	text := strings.ToLower(event.Summary + "\n" + event.Description)
	return strings.Contains(text, "[fixed]") || strings.Contains(text, "#fixed") || strings.Contains(text, "[фикс]")
}

func calendarDateTime(value *gcalendar.EventDateTime) string {
	if value == nil {
		return ""
	}
	if value.DateTime != "" {
		return value.DateTime
	}
	return value.Date
}
