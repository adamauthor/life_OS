drop index if exists notification_logs_user_action_idx;
drop index if exists notification_logs_user_created_at_idx;
drop table if exists notification_logs;

drop index if exists scheduled_notifications_user_status_idx;
drop index if exists scheduled_notifications_due_idx;
drop table if exists scheduled_notifications;

drop index if exists notification_rules_user_enabled_idx;
drop table if exists notification_rules;

drop table if exists user_autonomy_settings;

drop index if exists users_last_seen_at_idx;
drop table if exists users;
