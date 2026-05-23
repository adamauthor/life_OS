# Google Calendar Setup

The bot uses Google Calendar only when OAuth files are configured.

Required environment variables:

- `GOOGLE_CREDENTIALS_FILE`: OAuth client JSON downloaded from Google Cloud.
- `GOOGLE_TOKEN_FILE`: OAuth token JSON for the target Google account.
- `GOOGLE_CALENDAR_ID`: calendar ID, usually `primary`.

The bot runtime expects `GOOGLE_TOKEN_FILE` to already exist. Generate it once with:

```sh
GOOGLE_CREDENTIALS_FILE=client_secret_google_calendar.json \
GOOGLE_TOKEN_FILE=google_token_calendar.json \
go run ./cmd/google-auth
```

If your OAuth client type is `Web application`, add this authorized redirect URI in Google Cloud:

```text
http://localhost:8085/oauth2callback
```

For a local-only setup, `Desktop app` OAuth credentials are also fine, but this helper uses the same localhost callback flow.

Calendar safety rules:

- Event creation is stored first as a pending `calendar_actions` row.
- Telegram inline `Да` confirms the write.
- `Нет` cancels the pending action.
- Replanning produces a structured plan proposal.
- Replanning mutates the calendar only after `Да`.
- Events marked with `[fixed]`, `#fixed`, or `[фикс]` in title/description are treated as fixed and are not updated by replan.
