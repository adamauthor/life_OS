# Life OS Runbook

## What This Bot Does

The bot accepts text and voice messages, classifies intent, saves memories, searches memory, runs daily/weekly reviews, extracts behavioral patterns, builds daily direction, and proposes day replans.

Hard rule: calendar writes happen only after explicit Telegram callback confirmation.

## Current Multi-User Status

Safe per-user data:

- memories;
- memory search;
- daily reviews;
- behavioral patterns;
- daily directions;
- replan proposals;
- pending calendar action ownership.

Current limitation:

- Google Calendar OAuth is still global for the whole bot instance.
- If calendar is configured, confirmed calendar actions write to the single configured calendar.
- For real public multi-user calendar support, add per-user Google OAuth token storage and a user calendar settings flow.

## Local Setup

Prerequisites:

- Go 1.25;
- Docker Desktop running;
- Telegram bot token from BotFather;
- OpenAI API key.

Prepare env:

```sh
cp .env.example .env
```

Edit `.env`:

```sh
DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable
OPENAI_API_KEY=sk-proj-...
TELEGRAM_BOT_TOKEN=123:abc
APP_TIMEZONE=Asia/Ho_Chi_Minh
GOOGLE_CALENDAR_ID=primary
```

Start Postgres:

```sh
docker compose up -d postgres
```

Run migrations:

```sh
go run ./cmd/migrate up
```

Check migration version:

```sh
go run ./cmd/migrate version
```

Run tests:

```sh
go test ./...
```

Run bot:

```sh
go run ./cmd/bot
```

## Google Calendar Locally

Calendar is optional. Without it, memory, voice, review, patterns, `/today`, and replan proposal generation still work, but calendar reads/writes will be limited.

Create OAuth token file:

```sh
GOOGLE_CREDENTIALS_FILE=client_secret_google_calendar.json \
GOOGLE_TOKEN_FILE=google_token_calendar.json \
go run ./cmd/google-auth
```

Then set in `.env`:

```sh
GOOGLE_CREDENTIALS_FILE=/absolute/path/client_secret_google_calendar.json
GOOGLE_TOKEN_FILE=/absolute/path/google_token_calendar.json
GOOGLE_CALENDAR_ID=primary
```

## Fly.io Setup

Detailed Fly instructions are in `docs/deploy-fly.md`.

Minimum flow:

```sh
fly auth login
fly apps create <app-name>
fly mpg create --name <db-name> --region sin --pgvector
fly mpg attach <cluster-id> --app <app-name>
fly secrets set --app <app-name> TELEGRAM_BOT_TOKEN='...' OPENAI_API_KEY='...'
fly deploy --remote-only --app <app-name>
```

The deploy runs `./life-os-migrate up` automatically through Fly `release_command`.

## Production Smoke Test

After deploy:

```sh
fly logs --app <app-name>
```

In Telegram:

1. Send `/start`.
2. Send a short memory: `идея: проверить Adaptive Life OS`.
3. Send `/search Adaptive Life OS`.
4. Send `/review`, then answer the review questions.
5. Send `/patterns`.
6. Send `/today`.
7. Send: `я проспал до 11:40, перестрой день`.
8. Confirm only if calendar action looks correct.

## Operational Commands

Run migrations locally:

```sh
go run ./cmd/migrate up
```

Rollback one migration locally:

```sh
go run ./cmd/migrate steps -1
```

Check production logs:

```sh
fly logs --app <app-name>
```

Check production migration version:

```sh
fly ssh console --app <app-name> -C './life-os-migrate version'
```

## Common Failures

`extension "vector" is not available`

Your Postgres does not have pgvector. Locally use `pgvector/pgvector:pg16`. On Fly create Managed Postgres with `--pgvector`.

`DATABASE_URL is required`

Set `.env` locally or attach Fly Postgres so Fly creates the `DATABASE_URL` secret.

`Календарь не настроен.`

Google Calendar env vars are missing or invalid. Use file vars locally and JSON vars on Fly.

`Не распознал голос.`

OpenAI audio transcription failed. Check `OPENAI_API_KEY`, network, and logs.

