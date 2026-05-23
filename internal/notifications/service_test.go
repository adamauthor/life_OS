package notifications

import (
	"testing"
	"time"

	"life_os/internal/domain"
)

func TestInQuietHoursAcrossMidnight(t *testing.T) {
	loc := time.FixedZone("test", 0)
	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{name: "before quiet", now: time.Date(2026, 5, 23, 23, 0, 0, 0, loc), want: false},
		{name: "after start", now: time.Date(2026, 5, 23, 23, 45, 0, 0, loc), want: true},
		{name: "after midnight", now: time.Date(2026, 5, 24, 2, 0, 0, 0, loc), want: true},
		{name: "after end", now: time.Date(2026, 5, 24, 8, 30, 0, 0, loc), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inQuietHours(tt.now, "23:30:00", "08:00:00"); got != tt.want {
				t.Fatalf("inQuietHours = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuleRunsTodayUsesISOWeekday(t *testing.T) {
	loc := time.FixedZone("test", 0)
	monday := time.Date(2026, 5, 25, 10, 0, 0, 0, loc)
	sunday := time.Date(2026, 5, 24, 10, 0, 0, 0, loc)

	weekly := domain.NotificationRule{ScheduleDOW: []int{1}}
	if !ruleRunsToday(weekly, monday) {
		t.Fatal("weekly rule should run on Monday")
	}
	if ruleRunsToday(weekly, sunday) {
		t.Fatal("weekly rule should not run on Sunday")
	}
}

func TestNormalizeClock(t *testing.T) {
	got, err := normalizeClock("22:30")
	if err != nil {
		t.Fatalf("normalizeClock returned error: %v", err)
	}
	if got != "22:30:00" {
		t.Fatalf("normalizeClock = %q, want 22:30:00", got)
	}

	if _, err := normalizeClock("night"); err == nil {
		t.Fatal("normalizeClock returned nil error for invalid time")
	}
}
