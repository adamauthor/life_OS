package app

import (
	"testing"

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
