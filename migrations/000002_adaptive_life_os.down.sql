drop index if exists replan_proposals_user_status_idx;
drop index if exists behavioral_patterns_user_confidence_idx;
drop index if exists daily_reviews_user_date_idx;
drop index if exists daily_directions_user_date_idx;
drop index if exists user_profile_user_id_idx;

alter table user_profile drop column if exists user_id;

drop table if exists replan_proposals;
drop table if exists behavioral_patterns;
drop table if exists daily_directions;

do $$
begin
    if exists (
        select 1
        from information_schema.tables
        where table_name = 'daily_reviews_legacy'
    ) then
        drop table if exists daily_reviews;
        alter table daily_reviews_legacy rename to daily_reviews;
    else
        drop table if exists daily_reviews;
    end if;
end $$;
