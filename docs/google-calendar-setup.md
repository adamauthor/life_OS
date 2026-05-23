# Google Calendar Setup

The bot supports per-user Google Calendar OAuth. Each Telegram user connects their own Google account with `/connect_calendar`.

Required environment variables:

- `GOOGLE_CREDENTIALS_FILE`: OAuth client JSON downloaded from Google Cloud for local use.
- `GOOGLE_CREDENTIALS_JSON`: same credentials as a JSON secret on Fly.
- `GOOGLE_OAUTH_REDIRECT_URL`: callback URL.
- `CALENDAR_TOKEN_ENCRYPTION_KEY`: long random secret used to encrypt stored OAuth tokens.
- `GOOGLE_CALENDAR_ID`: calendar ID, usually `primary`.
- `HTTP_ADDR`: local HTTP listener, usually `:8080`.

Local redirect URI:

```text
http://localhost:8080/oauth/google/callback
```

Fly redirect URI:

```text
https://<app-name>.fly.dev/oauth/google/callback
```

Local env:

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
/disconnect_calendar
```

The older single-token setup with `GOOGLE_TOKEN_JSON` or `GOOGLE_TOKEN_FILE` still exists for private/local fallback, but it is restricted to `CALENDAR_OWNER_TELEGRAM_ID`.

Calendar safety rules:

- Event creation is stored first as a pending `calendar_actions` row.
- Calendar reads/writes use the connected Telegram user's own Google token.
- Telegram inline `Да` confirms the write.
- `Нет` cancels the pending action.
- Replanning produces a structured plan proposal.
- Replanning mutates the calendar only after `Да`.
- Events marked with `[fixed]`, `#fixed`, or `[фикс]` in title/description are treated as fixed and are not updated by replan.
