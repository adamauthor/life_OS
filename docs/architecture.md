# Adaptive Life Companion MVP Architecture

## Product Boundary

MVP is a self-hosted Telegram bot. It captures text and voice, stores personal memory, proposes calendar actions, searches memory, and runs reviews. It never mutates calendar state or sends external messages without explicit user confirmation.

## Runtime Shape

Use one Go service for the MVP:

- `cmd/bot`: process entrypoint.
- `internal/app`: Telegram update routing and use-case orchestration.
- `internal/domain`: domain entities, value objects, and invariant rules.
- `internal/telegram`: Telegram Bot API client and transport types.
- `internal/config`: environment configuration.
- `internal/memory`: memory capture, summarization, embeddings, and search.
- `internal/ai`: OpenAI clients for intent parsing, summaries, embeddings, and transcription.
- `internal/calendar`: Google Calendar read/write adapter.
- `internal/review`: daily, weekly, and monthly review flows.
- `internal/profile`: user profile, rules, goals, and scheduling preferences.
- `internal/storage`: PostgreSQL connection, repositories, and transactions.
- `migrations`: SQL schema managed as plain forward migrations.

Keep it a modular monolith. Microservices, agent frameworks, and LangChain are explicitly out of scope.

Engineering rules for adding code and features are defined in `docs/engineering-rules.md`.

## Data Flow

### Text Capture

1. Telegram update arrives.
2. Bot logs raw input.
3. Intent parser returns strict JSON.
4. For memory capture, the service stores raw text, summary, tags, type, source, metadata, and embedding.
5. Bot responds with a short confirmation and next step.

### Voice Capture

1. Telegram voice update arrives.
2. Bot downloads the file.
3. OpenAI Whisper transcribes audio.
4. Transcription goes through the same text intent pipeline.

### Calendar Proposal

1. Intent parser detects `create_calendar_event` or `replan_day`.
2. Service writes a pending row to `calendar_actions`.
3. Bot shows the proposal with inline buttons.
4. Only `Confirm` executes Google Calendar writes.
5. Result is logged back to `calendar_actions`.

For `replan_day`, the AI returns a structured proposal. Fixed events are kept unchanged. Existing movable events are patched and new blocks are created only after confirmation.

### Search

1. `/search` or `ask_memory` intent creates a query embedding.
2. PostgreSQL orders memories by vector distance.
3. The answer is generated from retrieved memory context.

### Daily Review

1. `/review` prompts the review questions.
2. User response is summarized by AI.
3. Summary, mood, energy, wins, failures, and patterns are saved to `daily_reviews`.
4. The review summary is also saved to `memories` as a `reflection`.

## Confirmation Model

All risky actions use a pending action record:

- `status=pending`: generated but not applied.
- `status=confirmed`: user approved it.
- `status=applied`: external system mutation succeeded.
- `status=cancelled`: user rejected it.
- `status=failed`: attempted but failed.

Calendar creation, update, deletion, and external messaging must go through this model.

## AI Contract

Intent extraction returns strict JSON only. The Go service validates:

- known `intent`;
- known memory `type`;
- confidence threshold;
- `requires_confirmation=true` for calendar writes;
- timezone-aware datetimes;
- reasonable duration.

Low-confidence or invalid output becomes `unknown` and asks the user one direct clarification.

## PostgreSQL

Use PostgreSQL with `pgvector`.

Core tables:

- `memories`
- `calendar_actions`
- `daily_reviews`
- `user_profile`
- `habits`
- `habit_logs`

Use repositories per aggregate. Keep SQL explicit.

## Deployment

Use Docker for local development and fly.io for deployment:

- app container with the Go bot;
- PostgreSQL with pgvector;
- secrets via environment variables;
- long polling for MVP simplicity.

Webhook mode can be added later if needed.

## Milestone Order

1. Telegram skeleton: implemented.
2. PostgreSQL and migrations: implemented.
3. AI intent parser: implemented.
4. Voice input: implemented.
5. Calendar proposal: implemented.
6. Calendar write after confirmation: implemented.
7. Replan day: implemented with confirmation before applying calendar updates.
8. Memory search: implemented.
9. Daily review: implemented.
