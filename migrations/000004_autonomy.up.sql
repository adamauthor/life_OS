create extension if not exists pgcrypto;

create table if not exists users (
    id uuid primary key,
    telegram_user_id bigint not null unique,
    default_chat_id bigint not null,
    username text not null default '',
    first_name text not null default '',
    last_name text not null default '',
    timezone text not null default 'Asia/Ho_Chi_Minh',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    last_seen_at timestamptz not null default now()
);

create index if not exists users_last_seen_at_idx on users (last_seen_at desc);

create table if not exists user_autonomy_settings (
    user_id uuid primary key references users(id) on delete cascade,
    enabled boolean not null default false,
    quiet_start time not null default '23:30',
    quiet_end time not null default '08:00',
    max_messages_per_day integer not null default 5,
    morning_time time not null default '09:30',
    midday_time time not null default '14:00',
    shutdown_time time not null default '23:00',
    review_time time not null default '22:30',
    weekly_time time not null default '10:00',
    allowed_types jsonb not null default '["daily_direction","midday_checkin","shutdown","daily_review","weekly_review","pattern_nudge"]'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists notification_rules (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    kind text not null,
    enabled boolean not null default true,
    schedule_time time,
    schedule_dow jsonb not null default '[]'::jsonb,
    payload jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (user_id, kind)
);

create index if not exists notification_rules_user_enabled_idx on notification_rules (user_id, enabled);

create table if not exists scheduled_notifications (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    chat_id bigint not null,
    kind text not null,
    notification_key text not null,
    due_at timestamptz not null,
    status text not null default 'pending',
    payload jsonb not null default '{}'::jsonb,
    attempts integer not null default 0,
    locked_at timestamptz,
    sent_at timestamptz,
    actioned_at timestamptz,
    last_error text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (user_id, notification_key)
);

create index if not exists scheduled_notifications_due_idx on scheduled_notifications (status, due_at);
create index if not exists scheduled_notifications_user_status_idx on scheduled_notifications (user_id, status, due_at desc);

create table if not exists notification_logs (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    scheduled_notification_id uuid references scheduled_notifications(id) on delete set null,
    kind text not null,
    action text not null,
    message_text text not null default '',
    metadata_json jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);

create index if not exists notification_logs_user_created_at_idx on notification_logs (user_id, created_at desc);
create index if not exists notification_logs_user_action_idx on notification_logs (user_id, action, created_at desc);
