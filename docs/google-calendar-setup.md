# Google Calendar Setup

The bot uses Google Calendar only when OAuth files are configured.

Required environment variables:

- `GOOGLE_CREDENTIALS_FILE`: OAuth client JSON downloaded from Google Cloud.
- `GOOGLE_TOKEN_FILE`: OAuth token JSON for the target Google account.
- `GOOGLE_CALENDAR_ID`: calendar ID, usually `primary`.

The MVP expects `GOOGLE_TOKEN_FILE` to already exist. This keeps the bot runtime simple and avoids interactive OAuth inside the Telegram process.

Calendar safety rules:

- Event creation is stored first as a pending `calendar_actions` row.
- Telegram inline `Да` confirms the write.
- `Нет` cancels the pending action.
- Replanning produces a structured plan proposal.
- Replanning mutates the calendar only after `Да`.
- Events marked with `[fixed]`, `#fixed`, or `[фикс]` in title/description are treated as fixed and are not updated by replan.
