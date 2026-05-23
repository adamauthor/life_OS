package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"life_os/internal/domain"
)

type DailyReviewRepository struct {
	db DB
}

func NewDailyReviewRepository(db DB) *DailyReviewRepository {
	return &DailyReviewRepository{db: db}
}

func (r *DailyReviewRepository) SaveDailyReview(ctx context.Context, review domain.DailyReview) (domain.DailyReview, error) {
	if review.UserID == (domain.UUID{}) {
		review.UserID = domain.UserIDFromTelegram(0)
	}
	reviewDate := review.ReviewDate
	if reviewDate.IsZero() {
		reviewDate = review.Date
	}
	if reviewDate.IsZero() {
		reviewDate = time.Now()
	}
	review.ReviewDate = reviewDate
	review.Date = reviewDate

	wins, err := json.Marshal(review.Wins)
	if err != nil {
		return domain.DailyReview{}, fmt.Errorf("marshal review wins: %w", err)
	}
	failures, err := json.Marshal(review.Failures)
	if err != nil {
		return domain.DailyReview{}, fmt.Errorf("marshal review failures: %w", err)
	}
	helped, err := json.Marshal(review.Helped)
	if err != nil {
		return domain.DailyReview{}, fmt.Errorf("marshal review helped: %w", err)
	}
	harmed, err := json.Marshal(review.Harmed)
	if err != nil {
		return domain.DailyReview{}, fmt.Errorf("marshal review harmed: %w", err)
	}
	tomorrowFocus, err := json.Marshal(review.TomorrowFocus)
	if err != nil {
		return domain.DailyReview{}, fmt.Errorf("marshal review tomorrow_focus: %w", err)
	}
	patterns, err := json.Marshal(review.Patterns)
	if err != nil {
		return domain.DailyReview{}, fmt.Errorf("marshal review patterns: %w", err)
	}

	const query = `
		insert into daily_reviews (user_id, review_date, raw_text, summary, wins, failures, helped, harmed, tomorrow_focus, patterns)
		values ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10::jsonb)
		on conflict (user_id, review_date) do update set
			raw_text = excluded.raw_text,
			summary = excluded.summary,
			wins = excluded.wins,
			failures = excluded.failures,
			helped = excluded.helped,
			harmed = excluded.harmed,
			tomorrow_focus = excluded.tomorrow_focus,
			patterns = excluded.patterns
		returning id, created_at
	`
	if err := r.db.QueryRow(
		ctx,
		query,
		review.UserID,
		reviewDate,
		review.RawText,
		review.Summary,
		string(wins),
		string(failures),
		string(helped),
		string(harmed),
		string(tomorrowFocus),
		string(patterns),
	).Scan(&review.ID, &review.CreatedAt); err != nil {
		return domain.DailyReview{}, fmt.Errorf("upsert daily review: %w", err)
	}
	return review, nil
}

func (r *DailyReviewRepository) ListDailyReviews(ctx context.Context, userID domain.UUID, since time.Time, limit int) ([]domain.DailyReview, error) {
	if limit <= 0 {
		limit = 7
	}
	const query = `
		select id, user_id, review_date, raw_text, coalesce(summary, ''), wins, failures, helped, harmed, tomorrow_focus, patterns, created_at
		from daily_reviews
		where user_id = $1
		  and review_date >= $2
		order by review_date desc
		limit $3
	`
	rows, err := r.query(ctx, query, userID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("query daily reviews: %w", err)
	}
	defer rows.Close()

	reviews := make([]domain.DailyReview, 0, limit)
	for rows.Next() {
		review, err := scanDailyReview(rows)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily reviews: %w", err)
	}
	return reviews, nil
}

func scanDailyReview(row pgx.Row) (domain.DailyReview, error) {
	var review domain.DailyReview
	var winsBytes, failuresBytes, helpedBytes, harmedBytes, tomorrowFocusBytes, patternsBytes []byte
	if err := row.Scan(
		&review.ID,
		&review.UserID,
		&review.ReviewDate,
		&review.RawText,
		&review.Summary,
		&winsBytes,
		&failuresBytes,
		&helpedBytes,
		&harmedBytes,
		&tomorrowFocusBytes,
		&patternsBytes,
		&review.CreatedAt,
	); err != nil {
		return domain.DailyReview{}, fmt.Errorf("scan daily review: %w", err)
	}
	review.Date = review.ReviewDate
	if err := json.Unmarshal(winsBytes, &review.Wins); err != nil {
		return domain.DailyReview{}, fmt.Errorf("unmarshal review wins: %w", err)
	}
	if err := json.Unmarshal(failuresBytes, &review.Failures); err != nil {
		return domain.DailyReview{}, fmt.Errorf("unmarshal review failures: %w", err)
	}
	if err := json.Unmarshal(helpedBytes, &review.Helped); err != nil {
		return domain.DailyReview{}, fmt.Errorf("unmarshal review helped: %w", err)
	}
	if err := json.Unmarshal(harmedBytes, &review.Harmed); err != nil {
		return domain.DailyReview{}, fmt.Errorf("unmarshal review harmed: %w", err)
	}
	if err := json.Unmarshal(tomorrowFocusBytes, &review.TomorrowFocus); err != nil {
		return domain.DailyReview{}, fmt.Errorf("unmarshal review tomorrow_focus: %w", err)
	}
	if err := json.Unmarshal(patternsBytes, &review.Patterns); err != nil {
		return domain.DailyReview{}, fmt.Errorf("unmarshal review patterns: %w", err)
	}
	return review, nil
}

func (r *DailyReviewRepository) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	type queryer interface {
		Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	}
	db, ok := r.db.(queryer)
	if !ok {
		return nil, fmt.Errorf("db does not support Query")
	}
	return db.Query(ctx, sql, args...)
}
