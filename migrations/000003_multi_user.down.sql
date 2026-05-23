drop index if exists calendar_actions_user_created_at_idx;
drop index if exists calendar_actions_user_status_idx;
alter table calendar_actions drop column if exists user_id;

drop index if exists memories_user_type_idx;
drop index if exists memories_user_created_at_idx;
alter table memories drop column if exists user_id;

drop index if exists habits_user_status_idx;
alter table habits drop column if exists user_id;
