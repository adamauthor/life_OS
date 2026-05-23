package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"life_os/internal/app"
)

type GoogleClient struct {
	calendarID string
	service    *gcalendar.Service
}

func NewGoogleClient(ctx context.Context, credentialsFile string, tokenFile string, calendarID string) (*GoogleClient, error) {
	if credentialsFile == "" || tokenFile == "" {
		return nil, fmt.Errorf("google credentials and token files are required")
	}
	credentials, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read google credentials: %w", err)
	}
	config, err := google.ConfigFromJSON(credentials, gcalendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("parse google credentials: %w", err)
	}
	token, err := tokenFromFile(tokenFile)
	if err != nil {
		return nil, err
	}
	httpClient := config.Client(ctx, token)
	service, err := gcalendar.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create google calendar service: %w", err)
	}
	if calendarID == "" {
		calendarID = "primary"
	}
	return &GoogleClient{calendarID: calendarID, service: service}, nil
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

func tokenFromFile(path string) (*oauth2.Token, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open google token: %w", err)
	}
	defer file.Close()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode google token: %w", err)
	}
	return &token, nil
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
