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
- per-user data isolation for core bot data;
- per-user Google Calendar OAuth connections;
- encrypted storage for new Google OAuth tokens when `CALENDAR_TOKEN_ENCRYPTION_KEY` is set.
- opt-in autonomy scheduler;
- proactive daily direction, check-ins, shutdown, review, weekly review, and pattern nudges;
- snooze/done/skip callbacks for proactive notifications.

## Critical Or Near-Critical Issues

### 1. Token Key Management Is Still Basic

Status: important before public launch.

New per-user Google Calendar tokens can be encrypted with `CALENDAR_TOKEN_ENCRYPTION_KEY`, but key rotation and incident response are not implemented yet.

Fix:

- add key rotation with versioned token wrappers;
- require `CALENDAR_TOKEN_ENCRYPTION_KEY` in production;
- revoke Google tokens on disconnect when possible;
- add an admin runbook for token incident response.

### 2. No Access Policy Yet

Status: important before public launch.

Any Telegram user who finds the bot can use OpenAI calls and database storage.

Fix options:

- private beta allowlist: `ALLOWED_TELEGRAM_USER_IDS`;
- public mode with rate limits;
- usage logging and abuse controls.

### 3. Autonomy Has A Daily Message Cap But No Cost Rate Limit

Status: cost and abuse risk.

Voice transcription, embeddings, and chat calls can generate cost.
Autonomy has `max_messages_per_day`, but natural user-triggered AI calls are not rate-limited yet.

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
3. Add token key rotation and Google token revocation on disconnect.
4. Improve `/replan` edit flow.
5. Add production observability: structured user/action logs, basic metrics.
6. Add integration tests for migrations against Postgres in CI.

## Public Multi-User Definition

The bot is public-ready only when:

- user data is isolated;
- OpenAI cost is rate-limited;
- calendar integrations are per-user or calendar features are disabled for users without integration;
- there is a basic abuse policy;
- logs are sufficient to debug failures without exposing private memory content.
