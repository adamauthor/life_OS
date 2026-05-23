create extension if not exists "uuid-ossp";

alter table memories
    add column if not exists user_id uuid;

update memories
set user_id = uuid_generate_v5(uuid_ns_oid(), 'telegram:' || (metadata_json ->> 'user_id'))
where user_id is null
  and metadata_json ? 'user_id'
  and metadata_json ->> 'user_id' <> '';

update memories
set user_id = '00000000-0000-0000-0000-000000000001'::uuid
where user_id is null;

alter table memories
    alter column user_id set not null;

create index if not exists memories_user_created_at_idx on memories (user_id, created_at desc);
create index if not exists memories_user_type_idx on memories (user_id, type);

alter table calendar_actions
    add column if not exists user_id uuid;

update calendar_actions
set user_id = '00000000-0000-0000-0000-000000000001'::uuid
where user_id is null;

alter table calendar_actions
    alter column user_id set not null;

create index if not exists calendar_actions_user_status_idx on calendar_actions (user_id, status);
create index if not exists calendar_actions_user_created_at_idx on calendar_actions (user_id, created_at desc);

alter table habits
    add column if not exists user_id uuid;

update habits
set user_id = '00000000-0000-0000-0000-000000000001'::uuid
where user_id is null;

alter table habits
    alter column user_id set not null;

create index if not exists habits_user_status_idx on habits (user_id, status);
