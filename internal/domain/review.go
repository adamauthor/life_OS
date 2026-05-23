package domain

import "time"

type DailyReview struct {
	ID        int64
	Date      time.Time
	RawText   string
	Summary   string
	Mood      string
	Energy    int
	Wins      []string
	Failures  []string
	Patterns  []string
	CreatedAt time.Time
}
