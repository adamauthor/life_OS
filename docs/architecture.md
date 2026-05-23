# Architecture

## Product Boundary

Adaptive Life Companion is a self-hosted Telegram-first Adaptive Life OS MVP.

It supports memory capture, voice input, semantic memory search, daily direction, adaptive replanning, reviews, behavioral patterns, autonomy reminders, and per-user Google Calendar integration.

Hard constraints:

- Go modular monolith.
- No Web UI.
- No mobile app.
- No microservices.
- No LangChain or agent framework.
- No autonomous calendar writes.
- Human override is mandatory.

## Runtime Shape

The service is one Go process:

- Telegram long polling for bot updates.
- HTTP callback endpoint only for Google OAuth: `/oauth/google/callback`.
- Background autonomy scheduler in the same process.
- PostgreSQL as source of truth.
- Google Calendar reads/writes through per-user OAuth tokens.
- OpenAI for intent parsing, transcription, summaries, embeddings, and memory answers.

## Package Map

- `cmd/bot`: composition root and process entrypoint.
- `cmd/migrate`: `golang-migrate` runner.
- `internal/app`: Telegram routing and application orchestration.
- `internal/ai`: OpenAI adapter and prompt contracts.
- `internal/calendar`: Google Calendar client, per-user OAuth service, and OAuth callback server.
- `internal/companion`: authority companion response formatting.
- `internal/config`: environment loading.
- `internal/domain`: domain models and value types.
- `internal/notifications`: autonomy scheduler and proactive notification logic.
- `internal/patterns`: behavioral pattern extraction/update/query logic.
- `internal/planning`: daily direction and replan services.
- `internal/review`: daily and weekly review services.
- `internal/storage`: PostgreSQL repositories.
- `internal/telegram`: Telegram Bot API adapter.
- `migrations`: `golang-migrate` up/down SQL migrations.

## Core Flows

### Text And Voice Intake

1. Telegram update arrives.
2. User is registered/upserted by Telegram ID.
3. Voice messages are downloaded and transcribed.
4. Text goes through intent parsing.
5. The bot routes to memory capture, calendar proposal, replan, review, weekly review, memory search, or fallback.

Commands are optional for normal use. Voice and natural text go through the same intent pipeline.

### Memory

1. Intent parser classifies memory type.
2. AI summarizes and tags the raw text.
3. Embedding is created.
4. Memory is stored with `user_id`, source metadata, raw text, summary, tags, and vector.
5. `/search` and `ask_memory` use vector search scoped to the same user.

### Per-User Calendar

1. User sends `/connect_calendar`.
2. Bot creates an OAuth state row in `oauth_states`.
3. User opens the Google OAuth URL.
4. Google redirects to `/oauth/google/callback`.
5. Bot exchanges code for token and stores the token in `user_integrations`.
6. Calendar reads and writes resolve a Google client by `user_id`.

If `CALENDAR_TOKEN_ENCRYPTION_KEY` is set, newly stored tokens are encrypted before saving.

### Calendar Confirmation

Calendar writes use pending records:

- `calendar_actions` for standalone event proposals.
- `replan_proposals` for adaptive replans.

Flow:

1. AI or parser proposes an action.
2. Action is saved as pending.
3. Bot shows inline buttons.
4. Calendar mutation runs only on confirm callback.
5. Status is updated to applied, cancelled, or failed.

Fixed events are not moved by replan. A Google Calendar event is treated as fixed if title or description contains:

```text
[fixed]
#fixed
[фикс]
```

### Daily Direction

`/today` builds a direction, not a minute-by-minute schedule.

Inputs:

- user profile fallback;
- recent memories;
- recent daily reviews;
- behavioral patterns;
- connected user calendar events.

Output:

- 3 to 5 anchors;
- 1 to 3 priorities;
- no automatic calendar writes.

### Replan

`/replan` and natural messages like `я проспал до 11:40, перестрой день` produce:

- fixed events;
- anchors;
- flexible blocks;
- recovery blocks;
- optional blocks;
- calendar actions only where important.

The proposal is applied only after `replan_confirm:{proposal_id}`.

### Reviews And Patterns

Daily review:

1. `/review` asks five questions.
2. User answers in one message.
3. Raw review is saved.
4. AI summary, wins, failures, helped, harmed, tomorrow focus, and detected patterns are extracted.
5. Behavioral pattern confidence is updated.
6. Review summary is saved as memory.

Weekly review analyzes the last 7 days of memories, reviews, patterns, habit logs, and calendar events.

### Autonomy

Autonomy is opt-in per user:

```text
/autonomy on
```

Scheduler behavior:

- creates daily pending notifications from rules;
- respects quiet hours;
- enforces max proactive messages per day;
- sends Telegram messages with done, snooze, skip, review, or replan callbacks;
- never applies calendar writes without confirm.

## Data Model

Important tables:

- `users`
- `memories`
- `calendar_actions`
- `daily_directions`
- `daily_reviews`
- `behavioral_patterns`
- `replan_proposals`
- `habits`
- `habit_logs`
- `user_autonomy_settings`
- `notification_rules`
- `scheduled_notifications`
- `notification_logs`
- `user_integrations`
- `oauth_states`

## Deployment

Local:

- Docker Compose Postgres with pgvector.
- `go run ./cmd/migrate up`.
- `go run ./cmd/bot`.

Production:

- Fly.io app.
- Fly Managed Postgres with pgvector.
- Dockerfile build.
- Fly `release_command` runs migrations.
- GitHub Actions runs tests and optional Fly deploy.

## Non-Goals

Not in this iteration:

- Web UI;
- mobile app;
- Obsidian sync;
- screen time tracking;
- Apple Health;
- multi-user SaaS billing;
- payments;
- social features;
- agent swarm;
- complex knowledge graph;
- microservices.
