# Runbook

## Current System

Adaptive Life Companion is one Go monolith that runs:

- Telegram long polling;
- Google OAuth callback endpoint at `/oauth/google/callback`;
- autonomy scheduler;
- PostgreSQL repositories;
- OpenAI and Google Calendar adapters.

Calendar rule: the bot may propose changes, but it never writes to a calendar without explicit inline confirmation.

## Environment

Required:

```sh
DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable
OPENAI_API_KEY=sk-proj-...
TELEGRAM_BOT_TOKEN=123:abc
APP_TIMEZONE=Asia/Ho_Chi_Minh
```

Optional per-user Google Calendar OAuth:

```sh
GOOGLE_CALENDAR_ID=primary
GOOGLE_CREDENTIALS_FILE=/absolute/path/client_secret_google_calendar.json
GOOGLE_CREDENTIALS_JSON=
GOOGLE_OAUTH_REDIRECT_URL=http://localhost:8080/oauth/google/callback
CALENDAR_TOKEN_ENCRYPTION_KEY=change-me-long-random-secret
HTTP_ADDR=:8080
```

Autonomy:

```sh
AUTONOMY_SCHEDULER_ENABLED=true
AUTONOMY_DEFAULT_ENABLED=false
```

Migrations:

```sh
MIGRATIONS_SOURCE=file://migrations
```

Legacy private calendar fallback still exists through `GOOGLE_TOKEN_FILE` or `GOOGLE_TOKEN_JSON`, but public use should use per-user `/connect_calendar`.

## Local Setup

1. Copy env:

```sh
cp .env.example .env
```

2. Start Postgres:

```sh
docker compose up -d postgres
```

3. Run migrations:

```sh
go run ./cmd/migrate up
```

4. Run tests:

```sh
go test ./...
```

5. Start bot:

```sh
go run ./cmd/bot
```

## Google Calendar Local Setup

Create a Google OAuth client in Google Cloud Console.

Authorized redirect URI:

```text
http://localhost:8080/oauth/google/callback
```

Set:

```sh
GOOGLE_CREDENTIALS_FILE=/absolute/path/client_secret_google_calendar.json
GOOGLE_OAUTH_REDIRECT_URL=http://localhost:8080/oauth/google/callback
CALENDAR_TOKEN_ENCRYPTION_KEY=change-me-long-random-secret
HTTP_ADDR=:8080
```

Then in Telegram:

```text
/connect_calendar
/calendar_status
```

Disconnect:

```text
/disconnect_calendar
```

## Commands

User commands:

- `/start`
- `/help`
- `/today`
- `/replan`
- `/review`
- `/weekly`
- `/patterns`
- `/autonomy`
- `/connect_calendar`
- `/calendar_status`
- `/disconnect_calendar`
- `/search <question>`
- `/schedule`
- `/capture`
- `/settings`

Autonomy commands:

```text
/autonomy on
/autonomy off
/autonomy status
/autonomy quiet 23:30 08:00
/autonomy limit 5
/autonomy time daily_review 22:30
```

Notification kinds for `/autonomy time`:

- `daily_direction`
- `midday_checkin`
- `pattern_nudge`
- `daily_review`
- `shutdown`
- `weekly_review`

## Production Smoke Test

After deploy:

```sh
fly logs --app <app-name>
```

In Telegram:

1. Send `/start`.
2. Click Google Calendar connect button.
3. Send `/calendar_status`.
4. Send a memory: `идея: проверить Adaptive Life OS`.
5. Send `/search Adaptive Life OS`.
6. Send `/review`, then answer all five questions.
7. Send `/patterns`.
8. Send `/today`.
9. Send `/autonomy on`.
10. Send: `я проспал до 11:40, перестрой день`.
11. Confirm calendar changes only if the proposal is correct.

## Operational Commands

Run migrations locally:

```sh
go run ./cmd/migrate up
```

Check migration version locally:

```sh
go run ./cmd/migrate version
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

## Data Safety

Per-user scoped data:

- memories;
- memory search;
- daily reviews;
- weekly review context;
- behavioral patterns;
- daily directions;
- replan proposals;
- calendar action ownership;
- autonomy settings and notifications;
- Google Calendar OAuth connections.

Calendar writes require explicit inline confirmation.

Set `CALENDAR_TOKEN_ENCRYPTION_KEY` before users connect calendars. Existing unencrypted tokens remain readable, but new tokens are stored encrypted.

## Common Failures

`extension "vector" is not available`

Use `pgvector/pgvector:pg16` locally. On Fly, create Managed Postgres with `--pgvector`.

`DATABASE_URL is required`

Set `.env` locally or attach Fly Postgres so Fly creates the `DATABASE_URL` secret.

`Календарь не настроен.`

Google OAuth env vars are missing or the user has not connected a calendar. Set credentials and redirect URL, then run `/connect_calendar`.

`calendar token is encrypted but CALENDAR_TOKEN_ENCRYPTION_KEY is not set`

The token was saved encrypted and the runtime key is missing. Restore the same `CALENDAR_TOKEN_ENCRYPTION_KEY`.

`Не распознал голос.`

OpenAI audio transcription failed. Check `OPENAI_API_KEY`, network, and logs.

`Не сохранил: ошибка памяти.`

Postgres is unavailable or migrations were not applied.
