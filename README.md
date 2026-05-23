# Adaptive Life Companion

Self-hosted Telegram bot MVP for memory capture, calendar proposals, voice input, adaptive replanning, and reviews.

## Run

```sh
docker compose up -d postgres

DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable \
go run ./cmd/migrate up

DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable \
OPENAI_API_KEY=sk-proj-example \
TELEGRAM_BOT_TOKEN=123:abc \
APP_TIMEZONE=Asia/Ho_Chi_Minh \
go run ./cmd/bot
```

Environment example: `.env.example`.

User guide: `docs/user-guide.md`.
Runbook: `docs/runbook.md`.
BotFather setup: `docs/botfather.md`.
Roadmap and risks: `docs/roadmap-and-risks.md`.

Google Calendar per-user OAuth is enabled when credentials and redirect URL are configured:

```sh
GOOGLE_CREDENTIALS_FILE=/absolute/path/oauth-client.json
GOOGLE_CALENDAR_ID=primary
GOOGLE_OAUTH_REDIRECT_URL=http://localhost:8080/oauth/google/callback
CALENDAR_TOKEN_ENCRYPTION_KEY=change-me-long-random-secret
HTTP_ADDR=:8080
```

Each Telegram user connects their own calendar with `/connect_calendar`. The older global-token mode still exists for local/private fallback, but requires `CALENDAR_OWNER_TELEGRAM_ID` so a public bot cannot read or write one owner's calendar for every Telegram user.

For Fly.io deployment, see `docs/deploy-fly.md`.

## Migrations

Migrations use `golang-migrate` format:

```sh
DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable \
go run ./cmd/migrate up

DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable \
go run ./cmd/migrate version
```

The Docker image includes `life-os-migrate`; Fly runs `./life-os-migrate up` as `release_command`.

Per-user Google Calendar OAuth uses `/connect_calendar`; no shared token file is required.

## Tests

```sh
go test ./...

DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable \
go test ./internal/storage -run TestMemoryRepositoryCreateMemoryIntegration -count=1
```

## Current Milestone

MVP milestones 1-9 are implemented as one modular monolith:

- Telegram long polling.
- `/start` and MVP commands.
- Text message intake.
- Voice-first input: voice messages go through Whisper, intent parsing, classification, and the same action pipeline without commands.
- PostgreSQL + pgvector via Docker Compose.
- SQL migrations via `golang-migrate`.
- AI intent parser with strict JSON output.
- Incoming non-command text messages are classified, summarized, embedded, and saved to `memories`.
- Voice messages are downloaded from Telegram, transcribed with Whisper, and sent through the same intent pipeline.
- Calendar event intents create pending `calendar_actions` with inline buttons.
- Calendar writes happen only after `Confirm`.
- `/today` and `/schedule` read Google Calendar when configured.
- Replan requests read the day calendar, produce a structured proposed plan, and mutate calendar events only after `Confirm`.
- `/search` performs semantic search over memory and answers with context.
- Daily review text is summarized, saved to `daily_reviews`, and stored in memory as a `reflection`.
- Memories, reviews, patterns, proposals, and calendar action approvals are scoped by Telegram user.
- `/autonomy on` enables proactive reminders with done/snooze/skip callbacks.

## Architecture

See `docs/architecture.md`.

## Engineering Rules

See `docs/engineering-rules.md`.
