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

type NotificationRepository struct {
	db DB
}

func NewNotificationRepository(db DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) RegisterTelegramUser(ctx context.Context, input domain.RegisterTelegramUserInput) (domain.User, error) {
	userID := domain.UserIDFromTelegram(input.TelegramUserID)
	timezone := input.Timezone
	if timezone == "" {
		timezone = "Asia/Ho_Chi_Minh"
	}
	const query = `
		insert into users (id, telegram_user_id, default_chat_id, username, first_name, last_name, timezone)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (telegram_user_id) do update set
			default_chat_id = excluded.default_chat_id,
			username = excluded.username,
			first_name = excluded.first_name,
			last_name = excluded.last_name,
			timezone = excluded.timezone,
			updated_at = now(),
			last_seen_at = now()
		returning id, telegram_user_id, default_chat_id, username, first_name, last_name, timezone, created_at, updated_at, last_seen_at
	`
	var user domain.User
	if err := r.db.QueryRow(ctx, query, userID, input.TelegramUserID, input.ChatID, input.Username, input.FirstName, input.LastName, timezone).Scan(
		&user.ID,
		&user.TelegramUserID,
		&user.DefaultChatID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.Timezone,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastSeenAt,
	); err != nil {
		return domain.User{}, fmt.Errorf("upsert telegram user: %w", err)
	}
	if err := r.ensureAutonomySettings(ctx, user.ID, input.DefaultAutonomyEnabled); err != nil {
		return domain.User{}, err
	}
	if err := r.EnsureDefaultNotificationRules(ctx, user.ID); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (r *NotificationRepository) GetAutonomySettings(ctx context.Context, userID domain.UUID) (domain.AutonomySettings, error) {
	const query = `
		select user_id, enabled, quiet_start::text, quiet_end::text, max_messages_per_day,
		       morning_time::text, midday_time::text, shutdown_time::text, review_time::text, weekly_time::text,
		       allowed_types, created_at, updated_at
		from user_autonomy_settings
		where user_id = $1
	`
	var settings domain.AutonomySettings
	var allowedTypesBytes []byte
	if err := r.db.QueryRow(ctx, query, userID).Scan(
		&settings.UserID,
		&settings.Enabled,
		&settings.QuietStart,
		&settings.QuietEnd,
		&settings.MaxMessagesPerDay,
		&settings.MorningTime,
		&settings.MiddayTime,
		&settings.ShutdownTime,
		&settings.ReviewTime,
		&settings.WeeklyTime,
		&allowedTypesBytes,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	); err != nil {
		return domain.AutonomySettings{}, fmt.Errorf("select autonomy settings: %w", err)
	}
	if err := json.Unmarshal(allowedTypesBytes, &settings.AllowedTypes); err != nil {
		return domain.AutonomySettings{}, fmt.Errorf("unmarshal autonomy allowed types: %w", err)
	}
	return settings, nil
}

func (r *NotificationRepository) SetAutonomyEnabled(ctx context.Context, userID domain.UUID, enabled bool) error {
	if err := r.ensureAutonomySettings(ctx, userID, false); err != nil {
		return err
	}
	const query = `
		update user_autonomy_settings
		set enabled = $2,
		    updated_at = now()
		where user_id = $1
	`
	if _, err := r.exec(ctx, query, userID, enabled); err != nil {
		return fmt.Errorf("update autonomy enabled: %w", err)
	}
	return nil
}

func (r *NotificationRepository) UpdateAutonomyQuietHours(ctx context.Context, userID domain.UUID, quietStart string, quietEnd string) error {
	if err := r.ensureAutonomySettings(ctx, userID, false); err != nil {
		return err
	}
	const query = `
		update user_autonomy_settings
		set quiet_start = $2::time,
		    quiet_end = $3::time,
		    updated_at = now()
		where user_id = $1
	`
	if _, err := r.exec(ctx, query, userID, quietStart, quietEnd); err != nil {
		return fmt.Errorf("update autonomy quiet hours: %w", err)
	}
	return nil
}

func (r *NotificationRepository) UpdateAutonomyDailyLimit(ctx context.Context, userID domain.UUID, limit int) error {
	if err := r.ensureAutonomySettings(ctx, userID, false); err != nil {
		return err
	}
	const query = `
		update user_autonomy_settings
		set max_messages_per_day = $2,
		    updated_at = now()
		where user_id = $1
	`
	if _, err := r.exec(ctx, query, userID, limit); err != nil {
		return fmt.Errorf("update autonomy daily limit: %w", err)
	}
	return nil
}

func (r *NotificationRepository) EnsureDefaultNotificationRules(ctx context.Context, userID domain.UUID) error {
	defaults := []domain.NotificationRule{
		{UserID: userID, Kind: domain.NotificationKindDailyDirection, Enabled: true, ScheduleTime: "09:30:00", ScheduleDOW: []int{1, 2, 3, 4, 5, 6, 7}},
		{UserID: userID, Kind: domain.NotificationKindMiddayCheckin, Enabled: true, ScheduleTime: "14:00:00", ScheduleDOW: []int{1, 2, 3, 4, 5, 6, 7}},
		{UserID: userID, Kind: domain.NotificationKindPatternNudge, Enabled: true, ScheduleTime: "17:30:00", ScheduleDOW: []int{1, 2, 3, 4, 5, 6, 7}},
		{UserID: userID, Kind: domain.NotificationKindDailyReview, Enabled: true, ScheduleTime: "22:30:00", ScheduleDOW: []int{1, 2, 3, 4, 5, 6, 7}},
		{UserID: userID, Kind: domain.NotificationKindShutdown, Enabled: true, ScheduleTime: "23:00:00", ScheduleDOW: []int{1, 2, 3, 4, 5, 6, 7}},
		{UserID: userID, Kind: domain.NotificationKindWeeklyReview, Enabled: true, ScheduleTime: "10:00:00", ScheduleDOW: []int{1}},
	}
	for _, rule := range defaults {
		if err := r.upsertNotificationRule(ctx, rule); err != nil {
			return err
		}
	}
	return nil
}

func (r *NotificationRepository) UpdateNotificationRuleTime(ctx context.Context, userID domain.UUID, kind domain.NotificationKind, scheduleTime string) error {
	if err := r.EnsureDefaultNotificationRules(ctx, userID); err != nil {
		return err
	}
	const query = `
		update notification_rules
		set schedule_time = $3::time,
		    updated_at = now()
		where user_id = $1
		  and kind = $2
	`
	tag, err := r.exec(ctx, query, userID, string(kind), scheduleTime)
	if err != nil {
		return fmt.Errorf("update notification rule time: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("notification rule %q not found", kind)
	}
	return nil
}

func (r *NotificationRepository) ListNotificationRules(ctx context.Context, userID domain.UUID) ([]domain.NotificationRule, error) {
	const query = `
		select id, user_id, kind, enabled, coalesce(schedule_time::text, ''), schedule_dow, payload, created_at, updated_at
		from notification_rules
		where user_id = $1
		order by kind
	`
	rows, err := r.query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query notification rules: %w", err)
	}
	defer rows.Close()

	var rules []domain.NotificationRule
	for rows.Next() {
		rule, err := scanNotificationRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification rules: %w", err)
	}
	return rules, nil
}

func (r *NotificationRepository) ListAutonomyUsers(ctx context.Context) ([]domain.AutonomyUser, error) {
	const query = `
		select u.id, u.telegram_user_id, u.default_chat_id, u.username, u.first_name, u.last_name, u.timezone,
		       u.created_at, u.updated_at, u.last_seen_at,
		       s.user_id, s.enabled, s.quiet_start::text, s.quiet_end::text, s.max_messages_per_day,
		       s.morning_time::text, s.midday_time::text, s.shutdown_time::text, s.review_time::text, s.weekly_time::text,
		       s.allowed_types, s.created_at, s.updated_at
		from users u
		join user_autonomy_settings s on s.user_id = u.id
		where s.enabled = true
		order by u.last_seen_at desc
	`
	rows, err := r.query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query autonomy users: %w", err)
	}
	defer rows.Close()

	var users []domain.AutonomyUser
	for rows.Next() {
		var user domain.User
		var settings domain.AutonomySettings
		var allowedTypesBytes []byte
		if err := rows.Scan(
			&user.ID,
			&user.TelegramUserID,
			&user.DefaultChatID,
			&user.Username,
			&user.FirstName,
			&user.LastName,
			&user.Timezone,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.LastSeenAt,
			&settings.UserID,
			&settings.Enabled,
			&settings.QuietStart,
			&settings.QuietEnd,
			&settings.MaxMessagesPerDay,
			&settings.MorningTime,
			&settings.MiddayTime,
			&settings.ShutdownTime,
			&settings.ReviewTime,
			&settings.WeeklyTime,
			&allowedTypesBytes,
			&settings.CreatedAt,
			&settings.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan autonomy user: %w", err)
		}
		if err := json.Unmarshal(allowedTypesBytes, &settings.AllowedTypes); err != nil {
			return nil, fmt.Errorf("unmarshal autonomy user allowed types: %w", err)
		}
		rules, err := r.ListNotificationRules(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		users = append(users, domain.AutonomyUser{User: user, Settings: settings, Rules: rules})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate autonomy users: %w", err)
	}
	return users, nil
}

func (r *NotificationRepository) UpsertScheduledNotification(ctx context.Context, notification domain.ScheduledNotification) (domain.ScheduledNotification, bool, error) {
	if notification.ID == uuid.Nil {
		notification.ID = uuid.New()
	}
	payload, err := json.Marshal(notification.Payload)
	if err != nil {
		return domain.ScheduledNotification{}, false, fmt.Errorf("marshal scheduled notification payload: %w", err)
	}
	const query = `
		insert into scheduled_notifications (id, user_id, chat_id, kind, notification_key, due_at, status, payload)
		values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
		on conflict (user_id, notification_key) do update set
			chat_id = excluded.chat_id,
			due_at = excluded.due_at,
			payload = excluded.payload,
			updated_at = now()
		where scheduled_notifications.status = 'pending'
		  and scheduled_notifications.actioned_at is null
		returning id, created_at, updated_at
	`
	if err := r.db.QueryRow(
		ctx,
		query,
		notification.ID,
		notification.UserID,
		notification.ChatID,
		string(notification.Kind),
		notification.NotificationKey,
		notification.DueAt,
		string(notification.Status),
		string(payload),
	).Scan(&notification.ID, &notification.CreatedAt, &notification.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ScheduledNotification{}, false, nil
		}
		return domain.ScheduledNotification{}, false, fmt.Errorf("insert scheduled notification: %w", err)
	}
	return notification, true, nil
}

func (r *NotificationRepository) ListDueNotifications(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledNotification, error) {
	if limit <= 0 {
		limit = 20
	}
	const query = `
		select id, user_id, chat_id, kind, notification_key, due_at, status, payload, attempts, locked_at, sent_at, actioned_at, last_error, created_at, updated_at
		from scheduled_notifications
		where status = 'pending'
		  and due_at <= $1
		order by due_at
		limit $2
	`
	rows, err := r.query(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("query due notifications: %w", err)
	}
	defer rows.Close()

	var notifications []domain.ScheduledNotification
	for rows.Next() {
		notification, err := scanScheduledNotification(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due notifications: %w", err)
	}
	return notifications, nil
}

func (r *NotificationRepository) ClaimScheduledNotification(ctx context.Context, notificationID domain.UUID) (bool, error) {
	const query = `
		update scheduled_notifications
		set status = 'sending',
		    locked_at = now(),
		    attempts = attempts + 1,
		    updated_at = now()
		where id = $1
		  and status = 'pending'
	`
	tag, err := r.exec(ctx, query, notificationID)
	if err != nil {
		return false, fmt.Errorf("claim scheduled notification: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *NotificationRepository) MarkNotificationSent(ctx context.Context, notificationID domain.UUID) error {
	const query = `
		update scheduled_notifications
		set status = 'sent',
		    sent_at = now(),
		    locked_at = null,
		    updated_at = now()
		where id = $1
	`
	if _, err := r.exec(ctx, query, notificationID); err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}

func (r *NotificationRepository) MarkNotificationFailed(ctx context.Context, notificationID domain.UUID, message string) error {
	const query = `
		update scheduled_notifications
		set status = case when attempts >= 3 then 'failed' else 'pending' end,
		    locked_at = null,
		    last_error = $2,
		    updated_at = now()
		where id = $1
	`
	if _, err := r.exec(ctx, query, notificationID, message); err != nil {
		return fmt.Errorf("mark notification failed: %w", err)
	}
	return nil
}

func (r *NotificationRepository) MarkNotificationActioned(ctx context.Context, userID domain.UUID, notificationID domain.UUID, status domain.NotificationStatus) error {
	const query = `
		update scheduled_notifications
		set status = $3,
		    actioned_at = now(),
		    updated_at = now()
		where user_id = $1
		  and id = $2
	`
	if _, err := r.exec(ctx, query, userID, notificationID, string(status)); err != nil {
		return fmt.Errorf("mark notification actioned: %w", err)
	}
	return nil
}

func (r *NotificationRepository) SnoozeNotification(ctx context.Context, userID domain.UUID, notificationID domain.UUID, dueAt time.Time) error {
	const query = `
		update scheduled_notifications
		set status = 'pending',
		    due_at = $3,
		    actioned_at = now(),
		    sent_at = null,
		    locked_at = null,
		    updated_at = now()
		where user_id = $1
		  and id = $2
	`
	if _, err := r.exec(ctx, query, userID, notificationID, dueAt); err != nil {
		return fmt.Errorf("snooze notification: %w", err)
	}
	return nil
}

func (r *NotificationRepository) CountSentNotifications(ctx context.Context, userID domain.UUID, from time.Time, to time.Time) (int, error) {
	const query = `
		select count(*)
		from notification_logs
		where user_id = $1
		  and action = 'sent'
		  and created_at >= $2
		  and created_at < $3
	`
	var count int
	if err := r.db.QueryRow(ctx, query, userID, from, to).Scan(&count); err != nil {
		return 0, fmt.Errorf("count sent notifications: %w", err)
	}
	return count, nil
}

func (r *NotificationRepository) LogNotification(ctx context.Context, log domain.NotificationLog) error {
	metadata, err := json.Marshal(log.Metadata)
	if err != nil {
		return fmt.Errorf("marshal notification log metadata: %w", err)
	}
	const query = `
		insert into notification_logs (user_id, scheduled_notification_id, kind, action, message_text, metadata_json)
		values ($1, $2, $3, $4, $5, $6::jsonb)
	`
	if _, err := r.exec(ctx, query, log.UserID, log.ScheduledNotificationID, string(log.Kind), log.Action, log.MessageText, string(metadata)); err != nil {
		return fmt.Errorf("insert notification log: %w", err)
	}
	return nil
}

func (r *NotificationRepository) GetScheduledNotificationForUser(ctx context.Context, userID domain.UUID, notificationID domain.UUID) (domain.ScheduledNotification, error) {
	const query = `
		select id, user_id, chat_id, kind, notification_key, due_at, status, payload, attempts, locked_at, sent_at, actioned_at, last_error, created_at, updated_at
		from scheduled_notifications
		where user_id = $1
		  and id = $2
	`
	return r.getScheduledNotification(ctx, query, userID, notificationID)
}

func (r *NotificationRepository) ensureAutonomySettings(ctx context.Context, userID domain.UUID, enabled bool) error {
	const query = `
		insert into user_autonomy_settings (user_id, enabled)
		values ($1, $2)
		on conflict (user_id) do nothing
	`
	if _, err := r.exec(ctx, query, userID, enabled); err != nil {
		return fmt.Errorf("ensure autonomy settings: %w", err)
	}
	return nil
}

func (r *NotificationRepository) upsertNotificationRule(ctx context.Context, rule domain.NotificationRule) error {
	scheduleDOW, err := json.Marshal(rule.ScheduleDOW)
	if err != nil {
		return fmt.Errorf("marshal notification rule schedule_dow: %w", err)
	}
	payload, err := json.Marshal(rule.Payload)
	if err != nil {
		return fmt.Errorf("marshal notification rule payload: %w", err)
	}
	const query = `
		insert into notification_rules (user_id, kind, enabled, schedule_time, schedule_dow, payload)
		values ($1, $2, $3, nullif($4, '')::time, $5::jsonb, $6::jsonb)
		on conflict (user_id, kind) do nothing
	`
	if _, err := r.exec(ctx, query, rule.UserID, string(rule.Kind), rule.Enabled, rule.ScheduleTime, string(scheduleDOW), string(payload)); err != nil {
		return fmt.Errorf("upsert notification rule %q: %w", rule.Kind, err)
	}
	return nil
}

func scanNotificationRule(row pgx.Row) (domain.NotificationRule, error) {
	var rule domain.NotificationRule
	var kind string
	var scheduleDOWBytes, payloadBytes []byte
	if err := row.Scan(
		&rule.ID,
		&rule.UserID,
		&kind,
		&rule.Enabled,
		&rule.ScheduleTime,
		&scheduleDOWBytes,
		&payloadBytes,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	); err != nil {
		return domain.NotificationRule{}, fmt.Errorf("scan notification rule: %w", err)
	}
	rule.Kind = domain.NotificationKind(kind)
	if err := json.Unmarshal(scheduleDOWBytes, &rule.ScheduleDOW); err != nil {
		return domain.NotificationRule{}, fmt.Errorf("unmarshal notification rule schedule_dow: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, &rule.Payload); err != nil {
		return domain.NotificationRule{}, fmt.Errorf("unmarshal notification rule payload: %w", err)
	}
	return rule, nil
}

func scanScheduledNotification(row pgx.Row) (domain.ScheduledNotification, error) {
	var notification domain.ScheduledNotification
	var kind, status string
	var payloadBytes []byte
	var lockedAt, sentAt, actionedAt sql.NullTime
	if err := row.Scan(
		&notification.ID,
		&notification.UserID,
		&notification.ChatID,
		&kind,
		&notification.NotificationKey,
		&notification.DueAt,
		&status,
		&payloadBytes,
		&notification.Attempts,
		&lockedAt,
		&sentAt,
		&actionedAt,
		&notification.LastError,
		&notification.CreatedAt,
		&notification.UpdatedAt,
	); err != nil {
		return domain.ScheduledNotification{}, fmt.Errorf("scan scheduled notification: %w", err)
	}
	notification.Kind = domain.NotificationKind(kind)
	notification.Status = domain.NotificationStatus(status)
	if lockedAt.Valid {
		notification.LockedAt = &lockedAt.Time
	}
	if sentAt.Valid {
		notification.SentAt = &sentAt.Time
	}
	if actionedAt.Valid {
		notification.ActionedAt = &actionedAt.Time
	}
	if err := json.Unmarshal(payloadBytes, &notification.Payload); err != nil {
		return domain.ScheduledNotification{}, fmt.Errorf("unmarshal scheduled notification payload: %w", err)
	}
	return notification, nil
}

func (r *NotificationRepository) getScheduledNotification(ctx context.Context, query string, args ...any) (domain.ScheduledNotification, error) {
	return scanScheduledNotification(r.db.QueryRow(ctx, query, args...))
}

func (r *NotificationRepository) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	type queryer interface {
		Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	}
	db, ok := r.db.(queryer)
	if !ok {
		return nil, fmt.Errorf("db does not support Query")
	}
	return db.Query(ctx, sql, args...)
}

func (r *NotificationRepository) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	type execer interface {
		Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	}
	db, ok := r.db.(execer)
	if !ok {
		return pgconn.CommandTag{}, fmt.Errorf("db does not support Exec")
	}
	return db.Exec(ctx, sql, args...)
}
