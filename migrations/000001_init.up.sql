create extension if not exists vector;

create table if not exists memories (
    id bigserial primary key,
    type text not null,
    raw_text text not null,
    summary text not null,
    tags text[] not null default '{}',
    source text not null,
    created_at timestamptz not null default now(),
    embedding vector(1536),
    metadata_json jsonb not null default '{}'::jsonb
);

create index if not exists memories_created_at_idx on memories (created_at desc);
create index if not exists memories_type_idx on memories (type);
create index if not exists memories_tags_idx on memories using gin (tags);
create index if not exists memories_metadata_json_idx on memories using gin (metadata_json);

create table if not exists calendar_actions (
    id bigserial primary key,
    action_type text not null,
    status text not null,
    proposed_payload jsonb not null,
    confirmed_at timestamptz,
    created_at timestamptz not null default now()
);

create index if not exists calendar_actions_status_idx on calendar_actions (status);
create index if not exists calendar_actions_created_at_idx on calendar_actions (created_at desc);

create table if not exists daily_reviews (
    id bigserial primary key,
    date date not null unique,
    raw_text text not null,
    summary text not null,
    mood text,
    energy integer,
    wins text[] not null default '{}',
    failures text[] not null default '{}',
    patterns text[] not null default '{}',
    created_at timestamptz not null default now()
);

create table if not exists user_profile (
    id bigserial primary key,
    timezone text not null,
    wake_time_target time,
    sleep_time_target time,
    work_start time,
    work_end time,
    fixed_meetings jsonb not null default '[]'::jsonb,
    goals_json jsonb not null default '{}'::jsonb,
    rules_json jsonb not null default '{}'::jsonb,
    personality_mode text not null default 'authority_companion',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists habits (
    id bigserial primary key,
    name text not null,
    target_frequency text not null,
    status text not null,
    created_at timestamptz not null default now()
);

create table if not exists habit_logs (
    id bigserial primary key,
    habit_id bigint not null references habits(id) on delete cascade,
    date date not null,
    value numeric,
    notes text,
    created_at timestamptz not null default now(),
    unique (habit_id, date)
);

create index if not exists habit_logs_date_idx on habit_logs (date desc);
