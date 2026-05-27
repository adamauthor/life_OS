create table if not exists user_profile_facts (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null,
    category text not null,
    key text not null,
    value text not null,
    confidence numeric not null default 0.8,
    source text not null default 'user',
    status text not null default 'active',
    created_at timestamp not null default now(),
    updated_at timestamp not null default now(),
    unique (user_id, category, key)
);

create index if not exists user_profile_facts_user_category_idx on user_profile_facts (user_id, category, status);

create table if not exists knowledge_items (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null,
    type text not null,
    title text not null,
    raw_text text not null,
    summary text not null,
    entities jsonb not null default '{}'::jsonb,
    amount numeric,
    currency text,
    due_date date,
    status text not null default 'active',
    tags jsonb not null default '[]'::jsonb,
    embedding vector(1536),
    created_at timestamp not null default now(),
    updated_at timestamp not null default now()
);

create index if not exists knowledge_items_user_type_status_idx on knowledge_items (user_id, type, status, created_at desc);
create index if not exists knowledge_items_due_date_idx on knowledge_items (user_id, due_date) where due_date is not null;
create index if not exists knowledge_items_entities_idx on knowledge_items using gin (entities);
create index if not exists knowledge_items_tags_idx on knowledge_items using gin (tags);

create table if not exists anchor_preferences (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null,
    anchor_code text not null,
    title text not null,
    preference_score numeric not null default 0,
    status text not null default 'neutral',
    reason text,
    last_feedback_at timestamp,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now(),
    unique (user_id, anchor_code)
);

create index if not exists anchor_preferences_user_status_idx on anchor_preferences (user_id, status, preference_score desc);
