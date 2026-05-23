# Google Calendar Setup

The bot supports per-user Google Calendar OAuth. Each Telegram user connects their own Google account with:

```text
/connect_calendar
```

Calendar reads and writes are then scoped to that Telegram user.

## Required Env

Local:

```sh
GOOGLE_CREDENTIALS_FILE=/absolute/path/client_secret_google_calendar.json
GOOGLE_OAUTH_REDIRECT_URL=http://localhost:8080/oauth/google/callback
CALENDAR_TOKEN_ENCRYPTION_KEY=change-me-long-random-secret
GOOGLE_CALENDAR_ID=primary
HTTP_ADDR=:8080
```

Fly:

```sh
GOOGLE_CREDENTIALS_JSON=<oauth-client-json>
GOOGLE_OAUTH_REDIRECT_URL=https://<app-name>.fly.dev/oauth/google/callback
CALENDAR_TOKEN_ENCRYPTION_KEY=<long-random-secret>
GOOGLE_CALENDAR_ID=primary
HTTP_ADDR=:8080
```

## Google Cloud Console

Create OAuth credentials:

1. Go to Google Cloud Console.
2. Enable Google Calendar API.
3. Create OAuth client.
4. Use Web application type.
5. Add redirect URI.

Local redirect URI:

```text
http://localhost:8080/oauth/google/callback
```

Fly redirect URI:

```text
https://<app-name>.fly.dev/oauth/google/callback
```

Download the OAuth client JSON.

## Telegram Flow

Connect:

```text
/connect_calendar
```

The bot sends an inline button with a Google OAuth link. The user opens it, grants Calendar access, and the callback stores that user's token in PostgreSQL. No user-specific Google token is configured through environment variables.

Check:

```text
/calendar_status
```

Disconnect:

```text
/disconnect_calendar
```

`/start` also sends a calendar connect button when OAuth is configured.

## Storage

Tables:

- `user_integrations`: stores per-user Google token and calendar ID.
- `oauth_states`: stores temporary OAuth state during connection.

When `CALENDAR_TOKEN_ENCRYPTION_KEY` is set, new stored tokens are encrypted with AES-GCM using a key derived from the secret.

Keep the key stable. If the key changes, previously encrypted tokens cannot be decrypted.

## Safety Rules

- Calendar reads use the connected user's token.
- Calendar writes require inline confirmation.
- Replan actions are proposals until the user confirms.
- Fixed events are not moved by replan.

Fixed markers:

```text
[fixed]
#fixed
[фикс]
```
