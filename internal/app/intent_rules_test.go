package app

import (
	"strings"
	"testing"
	"time"

	"life_os/internal/domain"
)

func TestNormalizeParsedIntentCorrectsCommonVoiceCases(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		model      domain.ParsedIntent
		wantIntent domain.Intent
	}{
		{
			name:       "replan",
			text:       "я проспал, сейчас 11:40 перестрой день",
			model:      domain.ParsedIntent{Intent: domain.IntentCaptureMemory, Type: domain.MemoryTypeNote},
			wantIntent: domain.IntentReplanDay,
		},
		{
			name:       "memory question",
			text:       "что я говорил про идею AI Life OS",
			model:      domain.ParsedIntent{Intent: domain.IntentCaptureMemory, Type: domain.MemoryTypeNote},
			wantIntent: domain.IntentAskMemory,
		},
		{
			name:       "calendar event",
			text:       "завтра в 11 разобрать Kafka consumer groups",
			model:      domain.ParsedIntent{Intent: domain.IntentCaptureMemory, Type: domain.MemoryTypeNote},
			wantIntent: domain.IntentCreateCalendarEvent,
		},
		{
			name:       "actual memory",
			text:       "идея: сделать сервис учета калорий как бюджет",
			model:      domain.ParsedIntent{Intent: domain.IntentCaptureMemory, Type: domain.MemoryTypeIdea},
			wantIntent: domain.IntentCaptureMemory,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeParsedIntent(tt.text, tt.model)
			if got.Intent != tt.wantIntent {
				t.Fatalf("Intent = %q, want %q", got.Intent, tt.wantIntent)
			}
		})
	}
}

func TestShouldCaptureAsMemory(t *testing.T) {
	for _, intent := range []domain.Intent{domain.IntentCreateCalendarEvent, domain.IntentReplanDay, domain.IntentAskMemory, domain.IntentUnknown} {
		if shouldCaptureAsMemory(intent) {
			t.Fatalf("shouldCaptureAsMemory(%q) = true, want false", intent)
		}
	}
}

func TestCalendarEventClarification(t *testing.T) {
	if got := calendarEventClarification(domain.ParsedIntent{
		Intent: domain.IntentCreateCalendarEvent,
		Title:  "Разобрать Kafka",
	}); !strings.Contains(got, "не хватает даты") {
		t.Fatalf("clarification = %q", got)
	}

	if got := calendarEventClarification(domain.ParsedIntent{
		Intent:   domain.IntentCreateCalendarEvent,
		Title:    "Разобрать Kafka",
		Datetime: "2026-05-24T11:00:00+07:00",
	}); got != "" {
		t.Fatalf("clarification = %q, want empty", got)
	}
}

func TestCompleteCalendarIntentFromTextUsesLocalRelativeTime(t *testing.T) {
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, loc)
	parsed := domain.ParsedIntent{
		Intent: domain.IntentCreateCalendarEvent,
		Type:   domain.MemoryTypeEvent,
	}

	got := completeCalendarIntentFromText("завтра в 11 разобрать Kafka consumer groups", parsed, now)

	if got.Datetime != "2026-05-24T11:00:00+07:00" {
		t.Fatalf("Datetime = %q, want 2026-05-24T11:00:00+07:00", got.Datetime)
	}
	if got.DurationMinutes != 60 {
		t.Fatalf("DurationMinutes = %d, want 60", got.DurationMinutes)
	}
	if got.Title == "" {
		t.Fatal("Title should be inferred")
	}
	if !got.RequiresConfirmation {
		t.Fatal("RequiresConfirmation should be true")
	}
}

func TestCompleteCalendarIntentFromTextOnlyTimeChoosesFutureLocalOccurrence(t *testing.T) {
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, loc)
	parsed := domain.ParsedIntent{Intent: domain.IntentCreateCalendarEvent}

	got := completeCalendarIntentFromText("в 11 созвон с Иваном", parsed, now)

	if got.Datetime != "2026-05-24T11:00:00+07:00" {
		t.Fatalf("Datetime = %q, want next day 11:00", got.Datetime)
	}
}
