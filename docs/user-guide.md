# User Guide

Adaptive Life Companion is designed for normal Telegram text and voice. Commands exist for explicit actions, but daily use should not require command syntax.

## First Start

Send:

```text
/start
```

The bot returns the basic guide and, when Google OAuth is configured, a button to connect Google Calendar.

## Calendar Connection

Connect your own calendar:

```text
/connect_calendar
```

Check status:

```text
/calendar_status
```

Disconnect:

```text
/disconnect_calendar
```

The bot can read your calendar for `/today`, `/schedule`, and `/replan`. It writes only after you confirm an inline button.

## Natural Usage

You can send text or voice.

Examples:

```text
идея: сервис учета калорий как финансовый бюджет
```

```text
завтра в 11 разобрать Kafka consumer groups
```

```text
я проспал, сейчас 11:40, перестрой день
```

```text
что я говорил про AI Life OS
```

Voice messages go through transcription and the same intent routing.

## Memory

Send a thought, note, idea, task, or reflection:

```text
идея: сделать сервис для учета калорий как финансовый бюджет
```

The bot saves:

- raw text;
- summary;
- type;
- tags;
- source metadata;
- embedding for search.

Memory is scoped to your Telegram user.

## Search

Command:

```text
/search что я говорил про AI Life OS
```

Natural question:

```text
что я говорил про AI Life OS
```

The bot searches only your own memories.

## Calendar Events

Send:

```text
завтра в 11 разобрать Kafka consumer groups
```

Expected behavior:

1. Bot parses the event.
2. Bot creates a pending calendar action.
3. Bot shows inline buttons.
4. Calendar event is created only after confirmation.

Buttons:

- `Да`: apply calendar write.
- `Изменить`: asks for corrected event text.
- `Нет`: cancel.

## Daily Direction

Send:

```text
/today
```

The bot returns a direction, not a minute-by-minute schedule.

Expected output:

- 3 to 5 anchors;
- 1 to 3 priorities;
- no autonomous calendar writes;
- calendar-aware if your calendar is connected.

## Replan

Send text or voice:

```text
я проснулся в 11:40, лег в 4 утра. перестрой день
```

The bot:

- reads today's connected calendar events;
- keeps fixed events;
- creates a realistic plan;
- separates fixed, anchor, flexible, recovery, and optional blocks;
- proposes calendar actions;
- applies calendar changes only after confirm.

Mark fixed events in Google Calendar title or description:

```text
[fixed]
#fixed
[фикс]
```

## Daily Review

Start:

```text
/review
```

Questions:

```text
1. Что сделал?
2. Что слил?
3. Что помогло?
4. Что ухудшило день?
5. Что завтра обязательно?
```

Reply in one message. The bot saves raw review, summary, extracted fields, and patterns.

## Weekly Review

Send:

```text
/weekly
```

The bot analyzes the last 7 days of memories, reviews, patterns, habit logs, and connected calendar events.

## Patterns

Send:

```text
/patterns
```

The bot lists active behavioral patterns with confidence.

Example:

```text
1. isolation_after_work - confidence 0.82
2. late_sleep_loop - confidence 0.76
```

## Autonomy

Autonomy is opt-in:

```text
/autonomy on
```

Disable:

```text
/autonomy off
```

Status:

```text
/autonomy status
```

Settings:

```text
/autonomy quiet 23:30 08:00
/autonomy limit 5
/autonomy time daily_review 22:30
```

Autonomy can send:

- morning daily direction;
- midday check-in;
- pattern nudge;
- daily review reminder;
- shutdown reminder;
- weekly review.

Notification buttons:

- `Сделал`;
- `Отложить 30м`;
- `Отложить 2ч`;
- `Пропустить`;
- `Ответить` for review prompts;
- `Перестроить` for replan prompts.

Calendar writes still require explicit confirmation.

## Commands

- `/start`: start guide and calendar connect prompt.
- `/help`: commands and examples.
- `/today`: adaptive daily direction.
- `/replan`: day replan proposal.
- `/review`: daily review.
- `/weekly`: weekly review.
- `/patterns`: active behavioral patterns.
- `/autonomy`: autonomy settings.
- `/connect_calendar`: connect Google Calendar.
- `/calendar_status`: calendar connection status.
- `/disconnect_calendar`: disconnect Google Calendar.
- `/search <question>`: search memory.
- `/schedule`: today's calendar events.
- `/capture`: prompt for memory capture.
- `/settings`: settings guidance.

## Troubleshooting

`Календарь не настроен.`

OAuth is not configured or your account is not connected. Run `/connect_calendar`.

`Не распознал голос. Повтори текстом.`

Transcription failed. Check `OPENAI_API_KEY`, network, and Telegram file access.

`Не сохранил: ошибка памяти.`

PostgreSQL is unavailable or migrations did not run.

`Не разобрал намерение.`

The message was too ambiguous. Send a shorter, more explicit message.
