# Adaptive Life Companion

Telegram-first Adaptive Life OS MVP: memory, voice input, daily direction, adaptive replanning, daily/weekly reviews, behavioral patterns, autonomy reminders, and per-user Google Calendar OAuth.

Hard rule: the bot can propose calendar changes, but calendar writes happen only after explicit Telegram button confirmation.

## What Is Implemented

- Telegram text and voice input.
- OpenAI intent parsing, summaries, embeddings, memory answers, and transcription.
- PostgreSQL + pgvector storage.
- `golang-migrate` up/down migrations.
- Per-user memories, reviews, patterns, daily directions, replans, notification settings, and calendar integrations.
- Per-user Google Calendar OAuth through `/connect_calendar`.
- `/today`, `/replan`, `/review`, `/weekly`, `/patterns`, `/autonomy`.
- Proactive opt-in reminders with done, snooze, skip, review, and replan callbacks.
- Fly.io Docker deployment and GitHub Actions CI/deploy workflow.

## Local Run

Start Postgres:

```sh
docker compose up -d postgres
```

Run migrations:

```sh
DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable \
go run ./cmd/migrate up
```

Run the bot:

```sh
DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable \
OPENAI_API_KEY=sk-proj-example \
TELEGRAM_BOT_TOKEN=123:abc \
APP_TIMEZONE=Asia/Ho_Chi_Minh \
go run ./cmd/bot
```

Environment template: `.env.example`.

## Google Calendar

Per-user OAuth is enabled when Google credentials and a redirect URL are configured:

```sh
GOOGLE_CREDENTIALS_FILE=/absolute/path/client_secret_google_calendar.json
GOOGLE_CALENDAR_ID=primary
GOOGLE_OAUTH_REDIRECT_URL=http://localhost:8080/oauth/google/callback
CALENDAR_TOKEN_ENCRYPTION_KEY=change-me-long-random-secret
HTTP_ADDR=:8080
```

Add this authorized redirect URI in Google Cloud Console:

```text
http://localhost:8080/oauth/google/callback
```

Then each Telegram user connects their own calendar:

```text
/connect_calendar
```

## Fly.io

See [docs/deploy-fly.md](docs/deploy-fly.md).

The Docker image includes `life-os-migrate`; Fly runs:

```sh
./life-os-migrate up
```

as `release_command` before starting the bot.

## Tests

```sh
go test ./...
```

Optional storage integration test:

```sh
DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable \
go test ./internal/storage -run TestMemoryRepositoryCreateMemoryIntegration -count=1
```

## CI/CD

GitHub Actions runs one `CI/CD` pipeline:

1. `gofmt`, `go mod tidy` diff check, `go vet`, race tests, and `go build`.
2. PostgreSQL + pgvector migrations and storage integration test.
3. Docker image build.
4. Fly deploy after all checks pass.

Pushes to `main` deploy only when repository variable `FLY_DEPLOY_ENABLED=true`; manual workflow dispatch also deploys after checks pass.

## Documentation

- [User guide](docs/user-guide.md)
- [Runbook](docs/runbook.md)
- [Architecture](docs/architecture.md)
- [Google Calendar setup](docs/google-calendar-setup.md)
- [BotFather setup](docs/botfather.md)
- [Deploy to Fly.io](docs/deploy-fly.md)
- [Roadmap and risks](docs/roadmap-and-risks.md)
- [Engineering rules](docs/engineering-rules.md)
