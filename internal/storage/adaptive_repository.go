package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"life_os/internal/domain"
)

type AdaptiveRepository struct {
	db DB
}

func NewAdaptiveRepository(db DB) *AdaptiveRepository {
	return &AdaptiveRepository{db: db}
}

func (r *AdaptiveRepository) SaveDailyDirection(ctx context.Context, direction domain.DailyDirection) (domain.DailyDirection, error) {
	if direction.ID == uuid.Nil {
		direction.ID = uuid.New()
	}
	anchors, err := json.Marshal(direction.Anchors)
	if err != nil {
		return domain.DailyDirection{}, fmt.Errorf("marshal daily direction anchors: %w", err)
	}
	priorities, err := json.Marshal(direction.Priorities)
	if err != nil {
		return domain.DailyDirection{}, fmt.Errorf("marshal daily direction priorities: %w", err)
	}
	const query = `
		insert into daily_directions (id, user_id, date, direction_text, anchors, priorities)
		values ($1, $2, $3, $4, $5::jsonb, $6::jsonb)
		on conflict (user_id, date) do update set
			direction_text = excluded.direction_text,
			anchors = excluded.anchors,
			priorities = excluded.priorities
		returning id, created_at
	`
	if err := r.db.QueryRow(
		ctx,
		query,
		direction.ID,
		direction.UserID,
		direction.Date,
		direction.Text,
		string(anchors),
		string(priorities),
	).Scan(&direction.ID, &direction.CreatedAt); err != nil {
		return domain.DailyDirection{}, fmt.Errorf("upsert daily direction: %w", err)
	}
	return direction, nil
}

func (r *AdaptiveRepository) SaveReplanProposal(ctx context.Context, proposal domain.ReplanProposal) (domain.ReplanProposal, error) {
	if proposal.ID == uuid.Nil {
		proposal.ID = uuid.New()
	}
	plan, err := json.Marshal(proposal.ProposedPlan)
	if err != nil {
		return domain.ReplanProposal{}, fmt.Errorf("marshal proposed plan: %w", err)
	}
	actions, err := json.Marshal(proposal.CalendarActions)
	if err != nil {
		return domain.ReplanProposal{}, fmt.Errorf("marshal replan calendar actions: %w", err)
	}
	const query = `
		insert into replan_proposals (id, user_id, status, reason, proposed_plan, calendar_actions, authority_message, risk_detected)
		values ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8)
		returning id, created_at
	`
	if err := r.db.QueryRow(
		ctx,
		query,
		proposal.ID,
		proposal.UserID,
		proposal.Status,
		proposal.Reason,
		string(plan),
		string(actions),
		proposal.AuthorityMessage,
		proposal.RiskDetected,
	).Scan(&proposal.ID, &proposal.CreatedAt); err != nil {
		return domain.ReplanProposal{}, fmt.Errorf("insert replan proposal: %w", err)
	}
	return proposal, nil
}

func (r *AdaptiveRepository) GetReplanProposal(ctx context.Context, proposalID domain.UUID) (domain.ReplanProposal, error) {
	const query = `
		select id, user_id, status, reason, proposed_plan, calendar_actions, authority_message, risk_detected, created_at, confirmed_at
		from replan_proposals
		where id = $1
	`
	return r.getReplanProposal(ctx, query, proposalID)
}

func (r *AdaptiveRepository) GetReplanProposalForUser(ctx context.Context, userID domain.UUID, proposalID domain.UUID) (domain.ReplanProposal, error) {
	const query = `
		select id, user_id, status, reason, proposed_plan, calendar_actions, authority_message, risk_detected, created_at, confirmed_at
		from replan_proposals
		where user_id = $1
		  and id = $2
	`
	return r.getReplanProposal(ctx, query, userID, proposalID)
}

func (r *AdaptiveRepository) getReplanProposal(ctx context.Context, query string, args ...any) (domain.ReplanProposal, error) {
	var proposal domain.ReplanProposal
	var planBytes, actionBytes []byte
	if err := r.db.QueryRow(ctx, query, args...).Scan(
		&proposal.ID,
		&proposal.UserID,
		&proposal.Status,
		&proposal.Reason,
		&planBytes,
		&actionBytes,
		&proposal.AuthorityMessage,
		&proposal.RiskDetected,
		&proposal.CreatedAt,
		&proposal.ConfirmedAt,
	); err != nil {
		return domain.ReplanProposal{}, fmt.Errorf("select replan proposal: %w", err)
	}
	if err := json.Unmarshal(planBytes, &proposal.ProposedPlan); err != nil {
		return domain.ReplanProposal{}, fmt.Errorf("unmarshal proposed plan: %w", err)
	}
	if err := json.Unmarshal(actionBytes, &proposal.CalendarActions); err != nil {
		return domain.ReplanProposal{}, fmt.Errorf("unmarshal replan calendar actions: %w", err)
	}
	return proposal, nil
}

func (r *AdaptiveRepository) UpdateReplanProposalStatus(ctx context.Context, proposalID domain.UUID, status string, confirmedAt *time.Time) error {
	const query = `
		update replan_proposals
		set status = $2,
		    confirmed_at = coalesce($3, confirmed_at)
		where id = $1
	`
	if _, err := r.exec(ctx, query, proposalID, status, confirmedAt); err != nil {
		return fmt.Errorf("update replan proposal status: %w", err)
	}
	return nil
}

func (r *AdaptiveRepository) UpdateReplanProposalStatusForUser(ctx context.Context, userID domain.UUID, proposalID domain.UUID, status string, confirmedAt *time.Time) error {
	const query = `
		update replan_proposals
		set status = $3,
		    confirmed_at = coalesce($4, confirmed_at)
		where user_id = $1
		  and id = $2
	`
	if _, err := r.exec(ctx, query, userID, proposalID, status, confirmedAt); err != nil {
		return fmt.Errorf("update user replan proposal status: %w", err)
	}
	return nil
}

func (r *AdaptiveRepository) UpsertBehavioralPatterns(ctx context.Context, patterns []domain.BehavioralPattern) error {
	const query = `
		insert into behavioral_patterns (
			id, user_id, code, title, description, signals, outcomes, counter_actions, confidence, last_seen_at
		)
		values ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9, $10)
		on conflict (user_id, code) do update set
			title = excluded.title,
			description = excluded.description,
			signals = excluded.signals,
			outcomes = excluded.outcomes,
			counter_actions = excluded.counter_actions,
			confidence = least(0.99, greatest(0.01, (behavioral_patterns.confidence + excluded.confidence) / 2 + 0.05)),
			last_seen_at = excluded.last_seen_at,
			updated_at = now()
	`
	for _, pattern := range patterns {
		if pattern.ID == uuid.Nil {
			pattern.ID = uuid.New()
		}
		signals, err := json.Marshal(pattern.Signals)
		if err != nil {
			return fmt.Errorf("marshal pattern signals: %w", err)
		}
		outcomes, err := json.Marshal(pattern.Outcomes)
		if err != nil {
			return fmt.Errorf("marshal pattern outcomes: %w", err)
		}
		counterActions, err := json.Marshal(pattern.CounterActions)
		if err != nil {
			return fmt.Errorf("marshal pattern counter actions: %w", err)
		}
		if _, err := r.exec(
			ctx,
			query,
			pattern.ID,
			pattern.UserID,
			pattern.Code,
			pattern.Title,
			pattern.Description,
			string(signals),
			string(outcomes),
			string(counterActions),
			pattern.Confidence,
			pattern.LastSeenAt,
		); err != nil {
			return fmt.Errorf("upsert behavioral pattern %q: %w", pattern.Code, err)
		}
	}
	return nil
}

func (r *AdaptiveRepository) ListBehavioralPatterns(ctx context.Context, userID domain.UUID, limit int, minConfidence float64) ([]domain.BehavioralPattern, error) {
	if limit <= 0 {
		limit = 10
	}
	const query = `
		select id, user_id, code, title, description, signals, outcomes, counter_actions, confidence, last_seen_at, created_at, updated_at
		from behavioral_patterns
		where user_id = $1
		  and confidence >= $2
		order by confidence desc, updated_at desc
		limit $3
	`
	rows, err := r.query(ctx, query, userID, minConfidence, limit)
	if err != nil {
		return nil, fmt.Errorf("query behavioral patterns: %w", err)
	}
	defer rows.Close()

	patterns := make([]domain.BehavioralPattern, 0, limit)
	for rows.Next() {
		pattern, err := scanBehavioralPattern(rows)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate behavioral patterns: %w", err)
	}
	return patterns, nil
}

func (r *AdaptiveRepository) GetUserProfile(ctx context.Context, userID domain.UUID) (domain.UserProfile, error) {
	const query = `
		select user_id, timezone, coalesce(wake_time_target::text, ''), coalesce(sleep_time_target::text, ''),
		       coalesce(work_start::text, ''), coalesce(work_end::text, ''),
		       goals_json, rules_json, personality_mode
		from user_profile
		order by case when user_id = $1 then 0 else 1 end, id
		limit 1
	`
	var profile domain.UserProfile
	var goalsBytes, rulesBytes []byte
	if err := r.db.QueryRow(ctx, query, userID).Scan(
		&profile.UserID,
		&profile.Timezone,
		&profile.WakeTimeTarget,
		&profile.SleepTimeTarget,
		&profile.WorkStart,
		&profile.WorkEnd,
		&goalsBytes,
		&rulesBytes,
		&profile.PersonalityMode,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultUserProfile(userID), nil
		}
		return defaultUserProfile(userID), nil
	}
	if err := json.Unmarshal(goalsBytes, &profile.Goals); err != nil {
		return domain.UserProfile{}, fmt.Errorf("unmarshal profile goals: %w", err)
	}
	if err := json.Unmarshal(rulesBytes, &profile.Rules); err != nil {
		return domain.UserProfile{}, fmt.Errorf("unmarshal profile rules: %w", err)
	}
	if profile.UserID == uuid.Nil {
		profile.UserID = userID
	}
	return profile, nil
}

func (r *AdaptiveRepository) ListRecentMemories(ctx context.Context, userID domain.UUID, limit int) ([]domain.Memory, error) {
	if limit <= 0 {
		limit = 12
	}
	const query = `
		select id, user_id, type, raw_text, summary, tags, source, created_at, metadata_json
		from memories
		where user_id = $1
		order by created_at desc
		limit $2
	`
	rows, err := r.query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent memories: %w", err)
	}
	defer rows.Close()

	memories := make([]domain.Memory, 0, limit)
	for rows.Next() {
		var memory domain.Memory
		var memoryType string
		var metadataBytes []byte
		if err := rows.Scan(
			&memory.ID,
			&memory.UserID,
			&memoryType,
			&memory.RawText,
			&memory.Summary,
			&memory.Tags,
			&memory.Source,
			&memory.CreatedAt,
			&metadataBytes,
		); err != nil {
			return nil, fmt.Errorf("scan recent memory: %w", err)
		}
		memory.Type = domain.MemoryType(memoryType)
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &memory.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal recent memory metadata: %w", err)
			}
		}
		memories = append(memories, memory)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent memories: %w", err)
	}
	return memories, nil
}

func (r *AdaptiveRepository) ListHabitLogs(ctx context.Context, userID domain.UUID, since time.Time) ([]domain.HabitLog, error) {
	const query = `
		select habit_logs.date, habits.name, coalesce(habit_logs.value, 0), coalesce(habit_logs.notes, '')
		from habit_logs
		join habits on habits.id = habit_logs.habit_id
		where habits.user_id = $1
		  and habit_logs.date >= $2
		order by habit_logs.date desc, habits.name
	`
	rows, err := r.query(ctx, query, userID, since)
	if err != nil {
		return nil, fmt.Errorf("query habit logs: %w", err)
	}
	defer rows.Close()

	var logs []domain.HabitLog
	for rows.Next() {
		var log domain.HabitLog
		if err := rows.Scan(&log.Date, &log.Name, &log.Value, &log.Notes); err != nil {
			return nil, fmt.Errorf("scan habit log: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate habit logs: %w", err)
	}
	return logs, nil
}

func scanBehavioralPattern(row pgx.Row) (domain.BehavioralPattern, error) {
	var pattern domain.BehavioralPattern
	var signalsBytes, outcomesBytes, counterActionsBytes []byte
	var lastSeen sql.NullTime
	if err := row.Scan(
		&pattern.ID,
		&pattern.UserID,
		&pattern.Code,
		&pattern.Title,
		&pattern.Description,
		&signalsBytes,
		&outcomesBytes,
		&counterActionsBytes,
		&pattern.Confidence,
		&lastSeen,
		&pattern.CreatedAt,
		&pattern.UpdatedAt,
	); err != nil {
		return domain.BehavioralPattern{}, fmt.Errorf("scan behavioral pattern: %w", err)
	}
	if lastSeen.Valid {
		pattern.LastSeenAt = &lastSeen.Time
	}
	if err := json.Unmarshal(signalsBytes, &pattern.Signals); err != nil {
		return domain.BehavioralPattern{}, fmt.Errorf("unmarshal pattern signals: %w", err)
	}
	if err := json.Unmarshal(outcomesBytes, &pattern.Outcomes); err != nil {
		return domain.BehavioralPattern{}, fmt.Errorf("unmarshal pattern outcomes: %w", err)
	}
	if err := json.Unmarshal(counterActionsBytes, &pattern.CounterActions); err != nil {
		return domain.BehavioralPattern{}, fmt.Errorf("unmarshal pattern counter actions: %w", err)
	}
	return pattern, nil
}

func defaultUserProfile(userID domain.UUID) domain.UserProfile {
	return domain.UserProfile{
		UserID:          userID,
		Timezone:        "Asia/Ho_Chi_Minh",
		Goals:           map[string]any{},
		Rules:           map[string]any{},
		PersonalityMode: "authority_companion",
	}
}

func (r *AdaptiveRepository) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	type queryer interface {
		Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	}
	db, ok := r.db.(queryer)
	if !ok {
		return nil, fmt.Errorf("db does not support Query")
	}
	return db.Query(ctx, sql, args...)
}

func (r *AdaptiveRepository) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	type execer interface {
		Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	}
	db, ok := r.db.(execer)
	if !ok {
		return pgconn.CommandTag{}, fmt.Errorf("db does not support Exec")
	}
	return db.Exec(ctx, sql, args...)
}
