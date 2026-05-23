# Engineering Rules

These rules define how new features and code are added to Adaptive Life Companion.

## Core Rule

Do not reinvent solved infrastructure.

Use mature, maintained libraries for protocol clients, persistence plumbing, migrations, OAuth, Telegram API, Google Calendar API, OpenAI API, logging, configuration parsing, validation, and test support when they are a good fit.

Write custom code for product-specific behavior: memory capture, confirmation policy, scheduling decisions, user profile rules, intent orchestration, and domain workflows.

## Product Safety Rules

- Human override is mandatory.
- Calendar writes, event deletion, important event creation, and external messages require explicit confirmation.
- AI output is treated as untrusted input and must be validated before use.
- Low confidence intent parsing must ask one direct clarification.
- Store pending external actions before applying them.
- Preserve raw user input whenever possible.
- Never hide destructive actions behind convenience automation.

## Architecture Style

Use a modular monolith.

The application is one deployable Go service with clear internal modules. Do not introduce microservices, queues, agent swarms, LangChain, or workflow engines for the MVP unless a concrete bottleneck appears.

Recommended package boundaries:

- `cmd/bot`: executable entrypoint only.
- `cmd/migrate`: migration runner.
- `internal/app`: use cases, orchestration, Telegram update routing.
- `internal/domain`: domain entities, value objects, domain rules.
- `internal/storage`: PostgreSQL repositories and transactions.
- `internal/telegram`: Telegram adapter.
- `internal/ai`: OpenAI adapter and prompt contracts.
- `internal/calendar`: Google Calendar adapter, OAuth service, callback server.
- `internal/config`: configuration loading and validation.
- `internal/planning`: daily direction and replan use cases.
- `internal/review`: review flows.
- `internal/patterns`: behavioral pattern logic.
- `internal/notifications`: autonomy scheduler and notification actions.
- `internal/companion`: authority companion response formatting.

Keep dependencies pointing inward:

- transport adapters depend on application interfaces;
- application layer depends on domain contracts;
- domain layer does not depend on Telegram, OpenAI, Google, PostgreSQL, or environment variables.

## Clean Architecture Rules

- Business rules must not live inside Telegram handlers.
- External API DTOs must not leak deep into domain code.
- Repositories are interfaces at the application/domain boundary when they improve testability or decouple external storage.
- Implementations of repositories live in infrastructure packages such as `internal/storage`.
- Use explicit transactions for workflows that write multiple related records.
- Keep side effects visible in use-case code.
- Do not let global state creep into feature logic.
- Prefer constructor injection over package-level mutable variables.

## DDD Rules

Use DDD pragmatically, not ceremonially.

Good aggregate candidates:

- `Memory`
- `CalendarAction`
- `DailyReview`
- `BehavioralPattern`
- `ReplanProposal`
- `ScheduledNotification`
- `UserIntegration`
- `Habit`

Rules:

- Entities own invariants that are always true.
- Value objects represent validated concepts such as `MemoryType`, `Intent`, `ActionStatus`, `UserTimezone`, and `DurationMinutes`.
- Aggregates should be small. Do not build a giant `User` aggregate that owns everything.
- Domain services are allowed for logic that does not naturally belong to one entity, such as schedule replanning.
- Application services coordinate repositories, AI, Telegram, and calendar adapters.
- Infrastructure packages translate between external formats and domain objects.

## Feature Development Flow

Every feature should follow this path:

1. Define the user behavior and confirmation requirements.
2. Add or update domain types and invariants.
3. Add application use case interfaces.
4. Implement adapters using established libraries.
5. Add storage schema and repository methods when persistence is needed.
6. Add focused tests around parsing, validation, and use-case behavior.
7. Wire the feature in `cmd/bot` or the appropriate composition root.
8. Update docs if behavior, setup, or architecture changes.

Do not start a feature by wiring external APIs directly into Telegram handlers.

## Library Selection

Prefer libraries that are:

- actively maintained;
- small enough to understand;
- idiomatic Go;
- compatible with context cancellation;
- easy to test;
- not framework-heavy.

Avoid libraries that:

- hide important side effects;
- impose a different architecture;
- require global mutable configuration;
- make simple flows harder to debug;
- duplicate features already solved by the Go standard library.

## Go Code Rules

- Keep functions short enough to read without scrolling whenever practical.
- Use clear names over clever names.
- Return errors with context using `%w`.
- Pass `context.Context` through IO and external API calls.
- Keep interfaces small and consumer-owned.
- Do not create interfaces only because an implementation exists.
- Prefer explicit structs over untyped maps.
- Avoid reflection unless a library requires it.
- Use standard library features before adding a dependency.
- Use `slog` for structured logs.
- Keep logs useful: include IDs, action status, intent, and external API names; do not log secrets.
- Avoid panic outside startup-time unrecoverable configuration errors.

## Database Rules

- Use PostgreSQL as the source of truth.
- Use `pgvector` for semantic search.
- Keep SQL migrations explicit and reviewable.
- Prefer simple SQL over complex ORM behavior.
- Store raw input and normalized metadata separately.
- Use JSONB for flexible metadata, not for core query fields.
- Add indexes when a query path becomes part of product behavior.
- Make status fields explicit enums at the application level.

## AI Rules

- All LLM intent extraction must return strict JSON.
- Validate model output before using it.
- Keep prompts versioned in code or prompt files.
- Separate intent extraction, summarization, embeddings, and final user response.
- Do not let the model decide whether confirmation is required for risky actions; code enforces that.
- Store model name and prompt version in metadata when useful.
- Treat AI as an assistant to the use case, not as the owner of the workflow.

## Calendar Rules

- Google Calendar writes must go through `CalendarAction`.
- Google Calendar clients must resolve by `user_id` when per-user OAuth is configured.
- Do not expose one global Google Calendar token to every Telegram user.
- A proposal is stored before confirmation.
- Confirmation applies exactly one pending action.
- Fixed events are read-only unless the user explicitly asks to change them.
- Replanning proposes changes first and applies them only after confirmation.
- Calendar adapter should be replaceable later without rewriting scheduling logic.
- Store OAuth tokens encrypted when `CALENDAR_TOKEN_ENCRYPTION_KEY` is configured.
- Never log OAuth tokens, authorization codes, refresh tokens, or calendar event payloads containing private content.

## Autonomy Rules

- Autonomy must be opt-in unless the deploy owner explicitly changes default behavior.
- Proactive messages must respect quiet hours.
- Proactive messages must respect a daily cap.
- Autonomy can ask, remind, nudge, and propose replans.
- Autonomy must not apply calendar writes.
- Every proactive notification should have an audit log and user action path.

## Testing Rules

- Unit test domain invariants and use cases.
- Use table-driven tests for parsers, classifiers, and validators.
- Mock external APIs through small interfaces.
- Integration tests may require Docker Compose once PostgreSQL is introduced.
- Add regression tests for bugs and risky confirmation paths.
- Do not chase 100% coverage; cover behavior that can break user trust.

## Simplicity Rules

- Build the smallest thing that completes the milestone correctly.
- Add abstractions only when they remove real duplication or protect a real boundary.
- Prefer boring code.
- Prefer explicit control flow.
- Keep package names concrete.
- Delete unused code quickly.
- Do not add future-proofing that has no near-term use.
- If code needs a long explanation, simplify the code first.

## Performance Rules

- Optimize for correctness and debuggability first.
- Avoid unnecessary network calls.
- Use batching where external APIs naturally support it.
- Add timeouts and context cancellation to IO.
- Keep embedding generation and search paths measurable.
- Do not introduce caching until there is a repeated expensive path.

## Documentation Rules

- Update `README.md` when setup or run commands change.
- Update `docs/architecture.md` when package boundaries or major workflows change.
- Update this document when a recurring engineering decision becomes a rule.
- Keep docs short enough that they are actually used.

## Review Checklist

Before considering a feature done:

- Does it preserve human confirmation for risky actions?
- Is business logic outside transport adapters?
- Is external API output validated?
- Are errors wrapped with useful context?
- Are secrets absent from logs?
- Are tests focused on the risky behavior?
- Did we use a library where the problem is already solved?
- Is the code still easy to read?
