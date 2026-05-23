# Roadmap And Risks

## Current Feature Status

Implemented:

- Telegram text input;
- Telegram voice input;
- memory capture;
- intent classification;
- PostgreSQL storage;
- OpenAI parsing and summaries;
- memory search;
- daily review;
- weekly review;
- behavioral pattern extraction and confidence updates;
- `/today` adaptive daily direction;
- `/replan` proposal with confirmation callbacks;
- Fly.io Docker deployment config;
- GitHub Actions CI and Fly deploy workflow;
- `golang-migrate` migrations;
- per-user data isolation for core bot data.

## Critical Or Near-Critical Issues

### 1. Per-User Google Calendar Is Not Implemented

Status: important product gap.

Core user data is now user-scoped, but Google Calendar OAuth is global per bot deployment. If multiple users confirm calendar actions, writes go to the configured calendar.

Fix:

- add `user_integrations` table;
- implement Google OAuth web callback or manual per-user token registration;
- store encrypted Google tokens per user;
- resolve calendar client by Telegram user;
- add `/connect_calendar`, `/calendar_status`, `/disconnect_calendar`.

### 2. No Access Policy Yet

Status: important before public launch.

Any Telegram user who finds the bot can use OpenAI calls and database storage.

Fix options:

- private beta allowlist: `ALLOWED_TELEGRAM_USER_IDS`;
- public mode with rate limits;
- usage logging and abuse controls.

### 3. No Rate Limiting

Status: cost and abuse risk.

Voice transcription, embeddings, and chat calls can generate cost.

Fix:

- per-user request limits;
- daily voice duration limits;
- max text length;
- log usage per user.

### 4. No Webhook Mode

Status: operational improvement, not blocker.

The bot uses long polling. That is fine for a monolith worker on Fly, but webhook mode can reduce latency and simplify observability.

Keep long polling for MVP unless there is a real scaling need.

### 5. Calendar Confirmation UX Needs Editing Flow

Status: feature gap.

`Изменить` currently asks the user to send corrected text, but does not keep a structured edit session.

Fix:

- store pending edit state;
- link correction to proposal ID;
- rebuild proposal from original + correction.

## Recommended Next Feature Order

1. Add `ALLOWED_TELEGRAM_USER_IDS` for beta safety.
2. Add per-user usage limits and message length limits.
3. Add per-user Google Calendar OAuth.
4. Improve `/replan` edit flow.
5. Add `/connect_calendar` and `/calendar_status`.
6. Add production observability: structured user/action logs, basic metrics.
7. Add integration tests for migrations against Postgres in CI.

## Public Multi-User Definition

The bot is public-ready only when:

- user data is isolated;
- OpenAI cost is rate-limited;
- calendar integrations are per-user or calendar features are disabled for users without integration;
- there is a basic abuse policy;
- logs are sufficient to debug failures without exposing private memory content.

