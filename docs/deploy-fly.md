# Deploy To Fly.io

The app runs as one Go process on Fly:

- Telegram long polling.
- Google OAuth HTTP callback at `/oauth/google/callback`.
- Autonomy scheduler.
- Migration binary executed as Fly `release_command`.

Sources:

- Fly deploy from GitHub Actions: https://fly.io/docs/blueprints/deploy-with-github-actions/
- Fly release command: https://fly.io/docs/reference/configuration/#the-deploy-section
- Fly Managed Postgres: https://fly.io/docs/mpg/create-and-connect/
- Fly Managed Postgres pgvector: https://fly.io/docs/mpg/overview/
- golang-migrate: https://github.com/golang-migrate/migrate

## 1. Install Tools

```sh
brew install flyctl
brew install go
fly auth login
```

Clone repo:

```sh
git clone <your-repo-url>
cd life_OS
```

## 2. Create App

Edit `fly.toml`:

```toml
app = "your-life-os-bot"
```

Create app:

```sh
fly apps create your-life-os-bot
```

## 3. Create Postgres

Create Fly Managed Postgres with pgvector:

```sh
fly mpg create --name your-life-os-db --region sin --pgvector
```

Attach it:

```sh
fly mpg attach <cluster-id-from-create-output> --app your-life-os-bot
```

This sets `DATABASE_URL` on the app.

## 4. Configure Google OAuth

In Google Cloud Console:

1. Create OAuth client credentials.
2. Use Web application client type.
3. Add authorized redirect URI:

```text
https://your-life-os-bot.fly.dev/oauth/google/callback
```

Download OAuth client JSON as `client_secret_google_calendar.json`.

## 5. Set Secrets

Required:

```sh
fly secrets set --app your-life-os-bot \
  TELEGRAM_BOT_TOKEN='123:abc' \
  OPENAI_API_KEY='sk-proj-...'
```

Recommended for per-user Google Calendar:

```sh
fly secrets set --app your-life-os-bot \
  GOOGLE_CREDENTIALS_JSON="$(cat client_secret_google_calendar.json)" \
  GOOGLE_CALENDAR_ID='primary' \
  GOOGLE_OAUTH_REDIRECT_URL='https://your-life-os-bot.fly.dev/oauth/google/callback' \
  CALENDAR_TOKEN_ENCRYPTION_KEY='paste-a-long-random-secret-here'
```

Keep `CALENDAR_TOKEN_ENCRYPTION_KEY` stable. Existing encrypted tokens require the same key.

## 6. Verify Locally

Start local Postgres:

```sh
docker compose up -d postgres
```

Run migrations:

```sh
DATABASE_URL='postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable' \
go run ./cmd/migrate up
```

Run tests:

```sh
go test ./...
```

## 7. Deploy

```sh
fly deploy --remote-only --app your-life-os-bot
```

Fly runs:

```sh
./life-os-migrate up
```

as `release_command`. If migrations fail, Fly does not roll out the new version.

## 8. GitHub Actions CI/CD

Create Fly deploy token:

```sh
fly tokens create deploy -x 999999h
```

In GitHub `Settings -> Secrets and variables -> Actions`:

- secret: `FLY_API_TOKEN=<token>`
- variable: `FLY_DEPLOY_ENABLED=true`

Workflow behavior:

- One `CI/CD` workflow runs on pull requests, pushes to `main`/`master`, and manual dispatch.
- `Format, Vet, Unit Tests` runs `gofmt`, `go mod tidy` diff check, `go vet ./...`, `go test -race ./...`, and `go build ./...`.
- `Migrations And Postgres Integration` starts `pgvector/pgvector:pg16`, runs migrations, checks migration version, runs storage integration tests, and verifies the latest down/up migration.
- `Docker Build` runs only after both test jobs pass.
- `Deploy To Fly` runs only after Docker build passes.
- Pushes to `main` deploy only when `FLY_DEPLOY_ENABLED=true`.
- Manual dispatch can deploy without changing `FLY_DEPLOY_ENABLED`.
- After deploy, the workflow runs `flyctl status`.

## 9. Smoke Test

Watch logs:

```sh
fly logs --app your-life-os-bot
```

In Telegram:

1. `/start`
2. `/connect_calendar`
3. `/calendar_status`
4. `/today`
5. `завтра в 11 разобрать Kafka consumer groups`
6. Confirm the event only if the proposal is correct.
7. `/autonomy on`

## 10. Useful Operations

Check migration version:

```sh
fly ssh console --app your-life-os-bot -C './life-os-migrate version'
```

Show releases:

```sh
fly releases --app your-life-os-bot
```

Rollback app image:

```sh
fly deploy --image <previous-image> --app your-life-os-bot
```

Do not run migration `down` in production unless you intentionally roll back schema and understand data loss risk.
