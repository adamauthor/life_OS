package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"life_os/internal/domain"
)

const googleCalendarProvider = "google_calendar"

type GoogleCalendarRepository struct {
	db DB
}

func NewGoogleCalendarRepository(db DB) *GoogleCalendarRepository {
	return &GoogleCalendarRepository{db: db}
}

func (r *GoogleCalendarRepository) SaveOAuthState(ctx context.Context, state domain.OAuthState) error {
	const query = `
		insert into oauth_states (state, user_id, provider, chat_id, expires_at)
		values ($1, $2, $3, $4, $5)
	`
	if _, err := r.exec(ctx, query, state.State, state.UserID, googleCalendarProvider, state.ChatID, state.ExpiresAt); err != nil {
		return fmt.Errorf("save oauth state: %w", err)
	}
	return nil
}

func (r *GoogleCalendarRepository) GetOAuthState(ctx context.Context, state string) (domain.OAuthState, error) {
	const query = `
		select state, user_id, provider, chat_id, expires_at, created_at
		from oauth_states
		where state = $1
	`
	var result domain.OAuthState
	if err := r.db.QueryRow(ctx, query, state).Scan(
		&result.State,
		&result.UserID,
		&result.Provider,
		&result.ChatID,
		&result.ExpiresAt,
		&result.CreatedAt,
	); err != nil {
		return domain.OAuthState{}, fmt.Errorf("select oauth state: %w", err)
	}
	return result, nil
}

func (r *GoogleCalendarRepository) DeleteOAuthState(ctx context.Context, state string) error {
	const query = `delete from oauth_states where state = $1`
	if _, err := r.exec(ctx, query, state); err != nil {
		return fmt.Errorf("delete oauth state: %w", err)
	}
	return nil
}

func (r *GoogleCalendarRepository) DeleteExpiredOAuthStates(ctx context.Context, now time.Time) error {
	const query = `delete from oauth_states where expires_at < $1`
	if _, err := r.exec(ctx, query, now); err != nil {
		return fmt.Errorf("delete expired oauth states: %w", err)
	}
	return nil
}

func (r *GoogleCalendarRepository) SaveGoogleCalendarConnection(ctx context.Context, connection domain.GoogleCalendarConnection) error {
	if !json.Valid([]byte(connection.TokenJSON)) {
		return fmt.Errorf("google token JSON is invalid")
	}
	const query = `
		insert into user_integrations (user_id, provider, calendar_id, token_json, connected_at)
		values ($1, $2, $3, $4::jsonb, now())
		on conflict (user_id, provider) do update set
			calendar_id = excluded.calendar_id,
			token_json = excluded.token_json,
			connected_at = now(),
			updated_at = now()
	`
	if _, err := r.exec(ctx, query, connection.UserID, googleCalendarProvider, connection.CalendarID, connection.TokenJSON); err != nil {
		return fmt.Errorf("save google calendar connection: %w", err)
	}
	return nil
}

func (r *GoogleCalendarRepository) UpdateGoogleCalendarToken(ctx context.Context, userID domain.UUID, tokenJSON string) error {
	if !json.Valid([]byte(tokenJSON)) {
		return fmt.Errorf("google token JSON is invalid")
	}
	const query = `
		update user_integrations
		set token_json = $3::jsonb,
		    updated_at = now()
		where user_id = $1
		  and provider = $2
	`
	tag, err := r.exec(ctx, query, userID, googleCalendarProvider, tokenJSON)
	if err != nil {
		return fmt.Errorf("update google calendar token: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("google calendar connection not found")
	}
	return nil
}

func (r *GoogleCalendarRepository) GetGoogleCalendarConnection(ctx context.Context, userID domain.UUID) (domain.GoogleCalendarConnection, error) {
	const query = `
		select id, user_id, calendar_id, token_json, connected_at, last_used_at, created_at, updated_at
		from user_integrations
		where user_id = $1
		  and provider = $2
	`
	var connection domain.GoogleCalendarConnection
	var tokenBytes []byte
	var lastUsedAt sql.NullTime
	if err := r.db.QueryRow(ctx, query, userID, googleCalendarProvider).Scan(
		&connection.ID,
		&connection.UserID,
		&connection.CalendarID,
		&tokenBytes,
		&connection.ConnectedAt,
		&lastUsedAt,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	); err != nil {
		return domain.GoogleCalendarConnection{}, fmt.Errorf("select google calendar connection: %w", err)
	}
	connection.TokenJSON = string(tokenBytes)
	if lastUsedAt.Valid {
		connection.LastUsedAt = &lastUsedAt.Time
	}
	return connection, nil
}

func (r *GoogleCalendarRepository) HasGoogleCalendarConnection(ctx context.Context, userID domain.UUID) (bool, error) {
	const query = `
		select exists (
			select 1
			from user_integrations
			where user_id = $1
			  and provider = $2
		)
	`
	var exists bool
	if err := r.db.QueryRow(ctx, query, userID, googleCalendarProvider).Scan(&exists); err != nil {
		return false, fmt.Errorf("check google calendar connection: %w", err)
	}
	return exists, nil
}

func (r *GoogleCalendarRepository) TouchGoogleCalendarConnection(ctx context.Context, userID domain.UUID) error {
	const query = `
		update user_integrations
		set last_used_at = now(),
		    updated_at = now()
		where user_id = $1
		  and provider = $2
	`
	if _, err := r.exec(ctx, query, userID, googleCalendarProvider); err != nil {
		return fmt.Errorf("touch google calendar connection: %w", err)
	}
	return nil
}

func (r *GoogleCalendarRepository) DeleteGoogleCalendarConnection(ctx context.Context, userID domain.UUID) error {
	const query = `
		delete from user_integrations
		where user_id = $1
		  and provider = $2
	`
	if _, err := r.exec(ctx, query, userID, googleCalendarProvider); err != nil {
		return fmt.Errorf("delete google calendar connection: %w", err)
	}
	return nil
}

func (r *GoogleCalendarRepository) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	type execer interface {
		Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	}
	db, ok := r.db.(execer)
	if !ok {
		return pgconn.CommandTag{}, fmt.Errorf("db does not support Exec")
	}
	return db.Exec(ctx, sql, args...)
}
