# Deploy to Fly.io

This project runs Telegram long polling and exposes one small HTTP callback endpoint for Google OAuth:

```text
/oauth/google/callback
```

Sources:
- Fly deploy from GitHub Actions: https://fly.io/docs/blueprints/deploy-with-github-actions/
- Fly release command: https://fly.io/docs/reference/configuration/#the-deploy-section
- Fly Managed Postgres: https://fly.io/docs/mpg/create-and-connect/
- Fly Managed Postgres pgvector: https://fly.io/docs/mpg/overview/
- golang-migrate: https://github.com/golang-migrate/migrate

## 1. Prepare local machine

Install:

```sh
brew install flyctl
brew install go
```

Log in:

```sh
fly auth login
```

Clone the repo and enter it:

```sh
git clone <your-repo-url>
cd life_OS
```

## 2. Create Fly app

Edit `fly.toml` and replace:

```toml
app = "life-os-bot"
```

Use a globally unique name:

```toml
app = "your-life-os-bot"
```

Create the app without deploying:

```sh
fly apps create your-life-os-bot
```

## 3. Create Fly Postgres

Create a Managed Postgres cluster with pgvector enabled:

```sh
fly mpg create --name your-life-os-db --region sin --pgvector
```

Attach it to the bot app:

```sh
fly mpg attach <cluster-id-from-create-output> --app your-life-os-bot
```

This sets `DATABASE_URL` as a Fly secret on the bot app.

## 4. Set secrets

Required:

```sh
fly secrets set --app your-life-os-bot \
  TELEGRAM_BOT_TOKEN='123:abc' \
  OPENAI_API_KEY='sk-proj-...'
```

Optional Google Calendar via JSON secrets:

```sh
fly secrets set --app your-life-os-bot \
  GOOGLE_CREDENTIALS_JSON="$(cat client_secret_google_calendar.json)" \
  GOOGLE_CALENDAR_ID='primary' \
  GOOGLE_OAUTH_REDIRECT_URL='https://your-life-os-bot.fly.dev/oauth/google/callback' \
  CALENDAR_TOKEN_ENCRYPTION_KEY='paste-a-long-random-secret-here'
```

In Google Cloud Console, add the same authorized redirect URI:

```text
https://your-life-os-bot.fly.dev/oauth/google/callback
```

After deploy, each Telegram user connects their own calendar with `/connect_calendar`.

Local file env vars like `GOOGLE_CREDENTIALS_FILE` are for local development. On Fly use JSON secrets.

## 5. Verify migrations locally

Start local Postgres:

```sh
docker compose up -d postgres
```

Run migrations:

```sh
DATABASE_URL='postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable' \
go run ./cmd/migrate up
```

Check version:

```sh
DATABASE_URL='postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable' \
go run ./cmd/migrate version
```

## 6. First manual deploy

Run:

```sh
fly deploy --remote-only --app your-life-os-bot
```

Fly will build the Docker image and run:

```sh
./life-os-migrate up
```

as `release_command` before starting the new bot process. If migrations fail, Fly will not roll out the new version.

## 7. GitHub Actions deploy

Create a Fly API token:

```sh
fly tokens create deploy -x 999999h
```

In GitHub repo settings:

1. Go to `Settings -> Secrets and variables -> Actions`.
2. Add repository secret:
   - `FLY_API_TOKEN=<token from fly tokens create>`
3. Add repository variable:
   - `FLY_DEPLOY_ENABLED=true`

Behavior:

- `CI` runs tests and Docker build on PRs and pushes to `main`/`master`.
- `Fly Deploy` can be started manually from the Actions tab.
- Pushes to `main` deploy only when `FLY_DEPLOY_ENABLED=true`.

## 8. Useful operations

Check app logs:

```sh
fly logs --app your-life-os-bot
```

Run migration version inside a temporary Fly machine:

```sh
fly ssh console --app your-life-os-bot -C './life-os-migrate version'
```

Rollback app release:

```sh
fly releases --app your-life-os-bot
fly deploy --image <previous-image> --app your-life-os-bot
```

Do not run `down` in production unless you are intentionally rolling back schema and understand the data loss risk.
