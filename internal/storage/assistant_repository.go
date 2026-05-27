package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pgvector/pgvector-go"

	"life_os/internal/domain"
)

type AssistantRepository struct {
	db DB
}

func NewAssistantRepository(db DB) *AssistantRepository {
	return &AssistantRepository{db: db}
}

func (r *AssistantRepository) UpsertUserProfileFact(ctx context.Context, fact domain.UserProfileFact) (domain.UserProfileFact, error) {
	if fact.ID == uuid.Nil {
		fact.ID = uuid.New()
	}
	if fact.Confidence == 0 {
		fact.Confidence = 0.8
	}
	if fact.Source == "" {
		fact.Source = "user"
	}
	if fact.Status == "" {
		fact.Status = "active"
	}
	const query = `
		insert into user_profile_facts (id, user_id, category, key, value, confidence, source, status)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (user_id, category, key) do update set
			value = excluded.value,
			confidence = excluded.confidence,
			source = excluded.source,
			status = excluded.status,
			updated_at = now()
		returning id, created_at, updated_at
	`
	if err := r.db.QueryRow(ctx, query, fact.ID, fact.UserID, fact.Category, fact.Key, fact.Value, fact.Confidence, fact.Source, fact.Status).Scan(&fact.ID, &fact.CreatedAt, &fact.UpdatedAt); err != nil {
		return domain.UserProfileFact{}, fmt.Errorf("upsert user profile fact: %w", err)
	}
	return fact, nil
}

func (r *AssistantRepository) ListUserProfileFacts(ctx context.Context, userID domain.UUID) ([]domain.UserProfileFact, error) {
	const query = `
		select id, user_id, category, key, value, confidence, source, status, created_at, updated_at
		from user_profile_facts
		where user_id = $1 and status = 'active'
		order by category, key
	`
	rows, err := r.query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user profile facts: %w", err)
	}
	defer rows.Close()

	var facts []domain.UserProfileFact
	for rows.Next() {
		var fact domain.UserProfileFact
		if err := rows.Scan(&fact.ID, &fact.UserID, &fact.Category, &fact.Key, &fact.Value, &fact.Confidence, &fact.Source, &fact.Status, &fact.CreatedAt, &fact.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user profile fact: %w", err)
		}
		facts = append(facts, fact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user profile facts: %w", err)
	}
	return facts, nil
}

func (r *AssistantRepository) CreateKnowledgeItem(ctx context.Context, item domain.KnowledgeItem) (domain.KnowledgeItem, error) {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.Status == "" {
		item.Status = "active"
	}
	entities, err := json.Marshal(item.Entities)
	if err != nil {
		return domain.KnowledgeItem{}, fmt.Errorf("marshal knowledge entities: %w", err)
	}
	tags, err := json.Marshal(item.Tags)
	if err != nil {
		return domain.KnowledgeItem{}, fmt.Errorf("marshal knowledge tags: %w", err)
	}
	var embedding any
	if len(item.Embedding) > 0 {
		embedding = pgvector.NewVector(item.Embedding)
	}
	const query = `
		insert into knowledge_items (id, user_id, type, title, raw_text, summary, entities, amount, currency, due_date, status, tags, embedding)
		values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12::jsonb, $13)
		returning id, created_at, updated_at
	`
	if err := r.db.QueryRow(ctx, query, item.ID, item.UserID, item.Type, item.Title, item.RawText, item.Summary, string(entities), item.Amount, nullString(item.Currency), item.DueDate, item.Status, string(tags), embedding).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return domain.KnowledgeItem{}, fmt.Errorf("insert knowledge item: %w", err)
	}
	return item, nil
}

func (r *AssistantRepository) ListKnowledgeItems(ctx context.Context, userID domain.UUID, itemType string, status string, limit int) ([]domain.KnowledgeItem, error) {
	if limit <= 0 {
		limit = 10
	}
	if status == "" {
		status = "active"
	}
	const query = `
		select id, user_id, type, title, raw_text, summary, entities, amount, currency, due_date, status, tags, created_at, updated_at
		from knowledge_items
		where user_id = $1
		  and ($2 = '' or type = $2)
		  and ($3 = '' or status = $3)
		order by coalesce(due_date, created_at::date), created_at desc
		limit $4
	`
	rows, err := r.query(ctx, query, userID, itemType, status, limit)
	if err != nil {
		return nil, fmt.Errorf("query knowledge items: %w", err)
	}
	defer rows.Close()
	return scanKnowledgeItems(rows)
}

func (r *AssistantRepository) SearchKnowledgeItems(ctx context.Context, userID domain.UUID, queryEmbedding []float32, limit int) ([]domain.KnowledgeItem, error) {
	if limit <= 0 {
		limit = 6
	}
	const query = `
		select id, user_id, type, title, raw_text, summary, entities, amount, currency, due_date, status, tags, created_at, updated_at
		from knowledge_items
		where user_id = $1
		  and embedding is not null
		  and status = 'active'
		order by embedding <-> $2
		limit $3
	`
	rows, err := r.query(ctx, query, userID, pgvector.NewVector(queryEmbedding), limit)
	if err != nil {
		return nil, fmt.Errorf("search knowledge items: %w", err)
	}
	defer rows.Close()
	return scanKnowledgeItems(rows)
}

func scanKnowledgeItems(rows pgx.Rows) ([]domain.KnowledgeItem, error) {
	var items []domain.KnowledgeItem
	for rows.Next() {
		var item domain.KnowledgeItem
		var entitiesBytes, tagsBytes []byte
		var amount sql.NullFloat64
		var currency sql.NullString
		var dueDate sql.NullTime
		if err := rows.Scan(&item.ID, &item.UserID, &item.Type, &item.Title, &item.RawText, &item.Summary, &entitiesBytes, &amount, &currency, &dueDate, &item.Status, &tagsBytes, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge item: %w", err)
		}
		if len(entitiesBytes) > 0 {
			if err := json.Unmarshal(entitiesBytes, &item.Entities); err != nil {
				return nil, fmt.Errorf("unmarshal knowledge entities: %w", err)
			}
		}
		if len(tagsBytes) > 0 {
			if err := json.Unmarshal(tagsBytes, &item.Tags); err != nil {
				return nil, fmt.Errorf("unmarshal knowledge tags: %w", err)
			}
		}
		if amount.Valid {
			item.Amount = &amount.Float64
		}
		if currency.Valid {
			item.Currency = currency.String
		}
		if dueDate.Valid {
			t := dueDate.Time
			item.DueDate = &t
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge items: %w", err)
	}
	return items, nil
}

func (r *AssistantRepository) UpsertAnchorPreference(ctx context.Context, pref domain.AnchorPreference) (domain.AnchorPreference, error) {
	if pref.ID == uuid.Nil {
		pref.ID = uuid.New()
	}
	if pref.Status == "" {
		pref.Status = "neutral"
	}
	now := time.Now()
	pref.LastFeedbackAt = &now
	const query = `
		insert into anchor_preferences (id, user_id, anchor_code, title, preference_score, status, reason, last_feedback_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (user_id, anchor_code) do update set
			title = excluded.title,
			preference_score = least(1, greatest(-1, anchor_preferences.preference_score + excluded.preference_score)),
			status = excluded.status,
			reason = excluded.reason,
			last_feedback_at = excluded.last_feedback_at,
			updated_at = now()
		returning id, preference_score, created_at, updated_at
	`
	if err := r.db.QueryRow(ctx, query, pref.ID, pref.UserID, pref.AnchorCode, pref.Title, pref.PreferenceScore, pref.Status, nullString(pref.Reason), pref.LastFeedbackAt).Scan(&pref.ID, &pref.PreferenceScore, &pref.CreatedAt, &pref.UpdatedAt); err != nil {
		return domain.AnchorPreference{}, fmt.Errorf("upsert anchor preference: %w", err)
	}
	return pref, nil
}

func (r *AssistantRepository) ListAnchorPreferences(ctx context.Context, userID domain.UUID) ([]domain.AnchorPreference, error) {
	const query = `
		select id, user_id, anchor_code, title, preference_score, status, coalesce(reason, ''), last_feedback_at, created_at, updated_at
		from anchor_preferences
		where user_id = $1
		order by preference_score desc, updated_at desc
	`
	rows, err := r.query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query anchor preferences: %w", err)
	}
	defer rows.Close()

	var prefs []domain.AnchorPreference
	for rows.Next() {
		var pref domain.AnchorPreference
		if err := rows.Scan(&pref.ID, &pref.UserID, &pref.AnchorCode, &pref.Title, &pref.PreferenceScore, &pref.Status, &pref.Reason, &pref.LastFeedbackAt, &pref.CreatedAt, &pref.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan anchor preference: %w", err)
		}
		prefs = append(prefs, pref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate anchor preferences: %w", err)
	}
	return prefs, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *AssistantRepository) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	type queryer interface {
		Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	}
	db, ok := r.db.(queryer)
	if !ok {
		return nil, fmt.Errorf("db does not support Query")
	}
	return db.Query(ctx, sql, args...)
}

func (r *AssistantRepository) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	type execer interface {
		Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	}
	db, ok := r.db.(execer)
	if !ok {
		return pgconn.CommandTag{}, fmt.Errorf("db does not support Exec")
	}
	return db.Exec(ctx, sql, args...)
}
