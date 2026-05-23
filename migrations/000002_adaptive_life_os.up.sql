create extension if not exists pgcrypto;

do $$
begin
    if exists (
        select 1
        from information_schema.columns
        where table_name = 'daily_reviews'
          and column_name = 'date'
    ) and not exists (
        select 1
        from information_schema.columns
        where table_name = 'daily_reviews'
          and column_name = 'review_date'
    ) then
        alter table daily_reviews rename to daily_reviews_legacy;
    end if;
end $$;

create table if not exists daily_directions (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null,
    date date not null,
    direction_text text not null,
    anchors jsonb not null default '[]'::jsonb,
    priorities jsonb not null default '[]'::jsonb,
    created_at timestamp not null default now(),
    unique (user_id, date)
);

create table if not exists daily_reviews (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null,
    review_date date not null,
    raw_text text not null,
    summary text,
    wins jsonb not null default '[]'::jsonb,
    failures jsonb not null default '[]'::jsonb,
    helped jsonb not null default '[]'::jsonb,
    harmed jsonb not null default '[]'::jsonb,
    tomorrow_focus jsonb not null default '[]'::jsonb,
    patterns jsonb not null default '[]'::jsonb,
    created_at timestamp not null default now(),
    unique (user_id, review_date)
);

do $$
begin
    if exists (
        select 1
        from information_schema.tables
        where table_name = 'daily_reviews_legacy'
    ) then
        insert into daily_reviews (
            user_id,
            review_date,
            raw_text,
            summary,
            wins,
            failures,
            patterns,
            created_at
        )
        select
            '00000000-0000-0000-0000-000000000001'::uuid,
            date,
            raw_text,
            summary,
            to_jsonb(wins),
            to_jsonb(failures),
            (
                select coalesce(jsonb_agg(jsonb_build_object(
                    'code', lower(regexp_replace(pattern_value, '[^a-zA-Z0-9]+', '_', 'g')),
                    'title', pattern_value,
                    'description', pattern_value,
                    'signals', '[]'::jsonb,
                    'outcomes', '[]'::jsonb,
                    'counter_actions', '[]'::jsonb,
                    'confidence', 0.5
                )), '[]'::jsonb)
                from unnest(patterns) as pattern_value
            ),
            created_at
        from daily_reviews_legacy
        on conflict (user_id, review_date) do nothing;
    end if;
end $$;

create table if not exists behavioral_patterns (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null,
    code text not null,
    title text not null,
    description text not null,
    signals jsonb not null default '[]'::jsonb,
    outcomes jsonb not null default '[]'::jsonb,
    counter_actions jsonb not null default '[]'::jsonb,
    confidence numeric not null default 0.5,
    last_seen_at timestamp,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now(),
    unique (user_id, code)
);

create table if not exists replan_proposals (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null,
    status text not null,
    reason text not null,
    proposed_plan jsonb not null,
    calendar_actions jsonb not null default '[]'::jsonb,
    authority_message text not null default '',
    risk_detected text not null default '',
    created_at timestamp not null default now(),
    confirmed_at timestamp
);

alter table user_profile
    add column if not exists user_id uuid not null default '00000000-0000-0000-0000-000000000001'::uuid;

create index if not exists user_profile_user_id_idx on user_profile (user_id);
create index if not exists daily_directions_user_date_idx on daily_directions (user_id, date desc);
create index if not exists daily_reviews_user_date_idx on daily_reviews (user_id, review_date desc);
create index if not exists behavioral_patterns_user_confidence_idx on behavioral_patterns (user_id, confidence desc);
create index if not exists replan_proposals_user_status_idx on replan_proposals (user_id, status, created_at desc);
