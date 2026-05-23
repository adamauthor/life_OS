package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"life_os/internal/domain"
)

type CalendarActionRepository struct {
	db DB
}

func NewCalendarActionRepository(db DB) *CalendarActionRepository {
	return &CalendarActionRepository{db: db}
}

func (r *CalendarActionRepository) CreateCalendarAction(ctx context.Context, action domain.CalendarAction) (domain.CalendarAction, error) {
	payload, err := json.Marshal(action.ProposedPayload)
	if err != nil {
		return domain.CalendarAction{}, fmt.Errorf("marshal calendar action payload: %w", err)
	}
	const query = `
		insert into calendar_actions (user_id, action_type, status, proposed_payload)
		values ($1, $2, $3, $4::jsonb)
		returning id, created_at
	`
	if err := r.db.QueryRow(ctx, query, action.UserID, action.ActionType, string(action.Status), string(payload)).Scan(&action.ID, &action.CreatedAt); err != nil {
		return domain.CalendarAction{}, fmt.Errorf("insert calendar action: %w", err)
	}
	return action, nil
}

func (r *CalendarActionRepository) GetCalendarAction(ctx context.Context, userID domain.UUID, id int64) (domain.CalendarAction, error) {
	const query = `
		select id, user_id, action_type, status, proposed_payload, confirmed_at, created_at
		from calendar_actions
		where user_id = $1
		  and id = $2
	`
	var action domain.CalendarAction
	var status string
	var payloadBytes []byte
	if err := r.db.QueryRow(ctx, query, userID, id).Scan(
		&action.ID,
		&action.UserID,
		&action.ActionType,
		&status,
		&payloadBytes,
		&action.ConfirmedAt,
		&action.CreatedAt,
	); err != nil {
		return domain.CalendarAction{}, fmt.Errorf("select calendar action: %w", err)
	}
	action.Status = domain.CalendarActionStatus(status)
	if err := json.Unmarshal(payloadBytes, &action.ProposedPayload); err != nil {
		return domain.CalendarAction{}, fmt.Errorf("unmarshal calendar action payload: %w", err)
	}
	return action, nil
}

func (r *CalendarActionRepository) UpdateCalendarActionStatus(ctx context.Context, userID domain.UUID, id int64, status domain.CalendarActionStatus) error {
	const query = `
		update calendar_actions
		set status = $2,
		    confirmed_at = case when $2 = 'confirmed' then $3 else confirmed_at end
		where id = $1
		  and user_id = $4
	`
	if _, err := r.exec(ctx, query, id, string(status), time.Now(), userID); err != nil {
		return fmt.Errorf("update calendar action status: %w", err)
	}
	return nil
}
