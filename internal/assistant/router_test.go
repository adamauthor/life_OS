package assistant

import (
	"testing"
	"time"
)

func TestRouteFallbackCommonVoicePhrases(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	tests := []struct {
		text string
		want AssistantIntent
	}{
		{text: "Что у меня сегодня?", want: IntentCalendarQuery},
		{text: "Запиши завтра в 15:00 созвон.", want: IntentCalendarCreate},
		{text: "Перенеси английский на вечер.", want: IntentCalendarUpdate},
		{text: "Я должен Куанышу 4,5 млн донгов.", want: IntentKnowledgeSave},
		{text: "Кому я должен?", want: IntentKnowledgeQuery},
		{text: "Моя цель на год — английский и senior Go.", want: IntentUserProfileUpdate},
		{text: "Что я хочу улучшить в этом году?", want: IntentUserProfileQuestion},
		{text: "Я не люблю плавать, не предлагай.", want: IntentAnchorFeedback},
		{text: "Сегодня поплавал, понравилось.", want: IntentAnchorFeedback},
		{text: "Что мне делать сегодня?", want: IntentTodayDirection},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := routeFallback(tt.text, now)
			if got.Intent != tt.want {
				t.Fatalf("Intent = %q, want %q", got.Intent, tt.want)
			}
		})
	}
}

func TestRouteFallbackDebtParsesAmount(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	got := routeFallback("Я должен Куанышу 4,5 млн донгов, вернуть до конца месяца.", now)
	if got.Knowledge == nil || got.Knowledge.Amount == nil {
		t.Fatal("expected parsed debt amount")
	}
	if *got.Knowledge.Amount != 4500000 {
		t.Fatalf("amount = %.0f, want 4500000", *got.Knowledge.Amount)
	}
	if got.Knowledge.Currency != "VND" {
		t.Fatalf("currency = %q, want VND", got.Knowledge.Currency)
	}
	if got.Knowledge.DueDate != "2026-05-31" {
		t.Fatalf("due_date = %q, want 2026-05-31", got.Knowledge.DueDate)
	}
}

func TestRouteFallbackCalendarCreateParsesRelativeTime(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	got := routeFallback("Запиши завтра в 15:00 созвон с Куанышем.", now)
	if got.Calendar == nil {
		t.Fatal("expected calendar payload")
	}
	if got.Calendar.StartTime != "2026-05-28T15:00:00+07:00" {
		t.Fatalf("start_time = %q, want 2026-05-28T15:00:00+07:00", got.Calendar.StartTime)
	}
	if got.Calendar.DurationMinutes != 60 {
		t.Fatalf("duration = %d, want 60", got.Calendar.DurationMinutes)
	}
}
