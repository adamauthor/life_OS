create table if not exists user_integrations (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    provider text not null,
    calendar_id text not null default 'primary',
    token_json jsonb not null,
    connected_at timestamptz not null default now(),
    last_used_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (user_id, provider)
);

create index if not exists user_integrations_user_provider_idx on user_integrations (user_id, provider);

create table if not exists oauth_states (
    state text primary key,
    user_id uuid not null references users(id) on delete cascade,
    provider text not null,
    chat_id bigint not null,
    expires_at timestamptz not null,
    created_at timestamptz not null default now()
);

create index if not exists oauth_states_expires_at_idx on oauth_states (expires_at);
