# Roadmap And Risks

## Implemented

Core:

- Telegram text input.
- Telegram voice input.
- Memory capture.
- Intent classification.
- PostgreSQL + pgvector.
- OpenAI parsing, summaries, embeddings, and transcription.
- Memory search.
- `golang-migrate` migrations.
- GitHub Actions CI.
- Fly.io deployment config.

Adaptive Life OS:

- Daily direction through `/today`.
- Adaptive replan through `/replan` and natural/voice intent.
- Daily review through `/review`.
- Weekly review through `/weekly`.
- Behavioral patterns through `/patterns`.
- Authority companion response style.

Multi-user:

- Per-user memories.
- Per-user reviews.
- Per-user patterns.
- Per-user daily directions.
- Per-user replan proposals.
- Per-user calendar action ownership.
- Per-user autonomy settings.
- Per-user Google Calendar OAuth through `/connect_calendar`.

Autonomy:

- Opt-in scheduler.
- Daily direction reminder.
- Midday check-in.
- Pattern nudge.
- Daily review reminder.
- Shutdown reminder.
- Weekly review.
- Done, snooze, skip, review, and replan callbacks.

Calendar safety:

- Calendar writes require explicit callback confirmation.
- Replan proposals do not apply before confirmation.
- Connected calendar is resolved by user.
- New Google OAuth tokens can be encrypted with `CALENDAR_TOKEN_ENCRYPTION_KEY`.

## Critical Risks

### 1. No Public Access Policy

Any Telegram user who finds the bot can consume OpenAI calls and database storage.

Fix:

- add `ALLOWED_TELEGRAM_USER_IDS` for private beta;
- or add public mode with per-user rate limits;
- log usage safely without private content;
- cap message length and voice duration.

### 2. No Usage Cost Limits

User-triggered OpenAI calls are not rate-limited. Autonomy has a daily proactive message cap, but normal text, voice, search, reviews, and replans can still create cost.

Fix:

- daily per-user request counters;
- voice duration limits;
- max text length;
- model usage logging.

### 3. Token Key Management Is Basic

New per-user Google Calendar tokens can be encrypted, but key rotation is not implemented.

Fix:

- require `CALENDAR_TOKEN_ENCRYPTION_KEY` in production;
- add versioned token wrappers;
- add rotation command or migration;
- revoke Google tokens on disconnect where possible;
- document incident response.

### 4. Edit Flow Is Shallow

`Изменить` currently asks the user to send corrected text, but does not keep a structured edit session tied to the proposal.

Fix:

- store pending edit state;
- link correction to action/proposal ID;
- rebuild proposal from original plus correction;
- expire stale edit sessions.

### 5. Limited Observability

Logs are structured but there are no product metrics or dashboards.

Fix:

- action counters;
- error counters by provider;
- notification delivery stats;
- migration/version visibility;
- privacy-safe user diagnostics.

## Recommended Next Order

1. Add beta allowlist or public rate limits.
2. Add usage counters and max input limits.
3. Improve calendar/replan edit sessions.
4. Add token revocation and key rotation.
5. Add production observability.
6. Add migration integration tests in CI with Postgres.

## Public Readiness Checklist

The bot is public-ready only when:

- user data remains isolated;
- OpenAI usage is rate-limited;
- calendar tokens are encrypted in production;
- Google token disconnect/revocation is handled;
- abuse policy exists;
- logs are sufficient for debugging without exposing memory content;
- BotFather description and commands are up to date;
- Fly secrets are configured and documented.
