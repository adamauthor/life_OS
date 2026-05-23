package storage

import (
	"context"
	"fmt"

	"life_os/internal/domain"
)

type DailyReviewRepository struct {
	db DB
}

func NewDailyReviewRepository(db DB) *DailyReviewRepository {
	return &DailyReviewRepository{db: db}
}

func (r *DailyReviewRepository) SaveDailyReview(ctx context.Context, review domain.DailyReview) (domain.DailyReview, error) {
	const query = `
		insert into daily_reviews (date, raw_text, summary, mood, energy, wins, failures, patterns)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (date) do update set
			raw_text = excluded.raw_text,
			summary = excluded.summary,
			mood = excluded.mood,
			energy = excluded.energy,
			wins = excluded.wins,
			failures = excluded.failures,
			patterns = excluded.patterns
		returning id, created_at
	`
	if err := r.db.QueryRow(
		ctx,
		query,
		review.Date,
		review.RawText,
		review.Summary,
		review.Mood,
		review.Energy,
		review.Wins,
		review.Failures,
		review.Patterns,
	).Scan(&review.ID, &review.CreatedAt); err != nil {
		return domain.DailyReview{}, fmt.Errorf("upsert daily review: %w", err)
	}
	return review, nil
}
