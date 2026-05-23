# User Guide

Adaptive Life Companion is designed to be used mostly through normal text and voice messages. Commands exist for explicit actions, but daily use should not require command syntax.

## First Run

1. Start PostgreSQL:

```sh
docker compose up -d postgres
```

2. Start the bot:

```sh
DATABASE_URL=postgres://life_os:life_os@localhost:5432/life_os?sslmode=disable \
OPENAI_API_KEY=sk-proj-example \
TELEGRAM_BOT_TOKEN=123:abc \
APP_TIMEZONE=Asia/Ho_Chi_Minh \
go run ./cmd/bot
```

3. Open Telegram and send:

```text
/start
```

The bot should answer that it is enabled.

## Environment

Required:

- `DATABASE_URL`: PostgreSQL connection string.
- `OPENAI_API_KEY`: OpenAI API key for intent parsing, Whisper, embeddings, and answers.
- `TELEGRAM_BOT_TOKEN`: Telegram bot token from BotFather.
- `APP_TIMEZONE`: user timezone, for example `Asia/Ho_Chi_Minh`.

Optional Google Calendar:

- `GOOGLE_CALENDAR_ID`: usually `primary`.
- `GOOGLE_CREDENTIALS_FILE`: Google OAuth client JSON.
- `GOOGLE_OAUTH_REDIRECT_URL`: OAuth callback URL.
- `CALENDAR_TOKEN_ENCRYPTION_KEY`: secret for encrypting stored OAuth tokens.
- `HTTP_ADDR`: callback listener, usually `:8080`.

If Google Calendar is not configured, memory, voice transcription, search, review, patterns, `/today`, `/weekly`, and `/replan` can still work. With Google OAuth configured, each user connects their own calendar:

```text
/connect_calendar
/calendar_status
/disconnect_calendar
```

## Autonomy

Autonomy is opt-in:

```text
/autonomy on
```

Useful settings:

```text
/autonomy quiet 23:30 08:00
/autonomy limit 5
/autonomy time daily_review 22:30
/autonomy status
```

The bot can send reminders, check-ins, review prompts, and replan proposals. It still cannot write to the calendar without inline confirmation.

## Main Rule

The bot can suggest, plan, warn, and prepare changes.

The bot must not write to the calendar without your explicit confirmation through inline buttons.

## Voice-First Usage

Send a Telegram voice message naturally. No command is required.

Examples:

```text
я проспал, сейчас 11:40, перестрой день
```

Expected flow:

1. Bot downloads the voice message.
2. Whisper transcribes it.
3. LLM detects `replan_day`.
4. Bot reads today calendar.
5. Bot proposes a new plan.
6. Bot shows `[Да] [Изменить] [Нет]`.
7. Calendar changes happen only after `[Да]`.

Other voice examples:

```text
идея: сервис учета калорий как финансовый бюджет
```

```text
завтра в 11 разобрать Kafka consumer groups
```

```text
что я говорил про AI Life OS
```

## Text Capture

Write thoughts directly:

```text
идея: сделать сервис для учета калорий как финансовый бюджет
```

The bot will:

- classify the memory type;
- save raw text;
- create a summary;
- assign tags;
- create an embedding;
- save it to PostgreSQL.

Supported memory types:

- `idea`
- `task`
- `note`
- `reflection`
- `event`
- `question`

## Calendar Events

Write naturally:

```text
завтра в 11 разобрать Kafka consumer groups
```

The bot should answer with a proposed event:

```text
Название: Разобрать Kafka consumer groups
Дата/время: ...
Длительность: 60 минут

Добавить в календарь?
[Да] [Изменить] [Нет]
```

Buttons:

- `Да`: create/update calendar after confirmation.
- `Изменить`: asks you to send corrected event text.
- `Нет`: cancels the pending action.

## Fixed Events

For replanning, fixed events should not move.

Mark fixed events in Google Calendar title or description with one of:

```text
[fixed]
#fixed
[фикс]
```

Examples:

```text
[fixed] Doctor appointment
```

```text
#fixed Team meeting
```

The bot will keep those events unchanged during replan.

## Replan Day

Send text or voice:

```text
я проснулся в 11:40, вчера лег в 4 утра. перестрой день
```

The bot will:

- read today calendar;
- keep fixed events;
- produce a structured plan;
- ask confirmation;
- update movable events and create new blocks only after confirmation.

## Search Memory

Use command:

```text
/search что я говорил про идею AI Life OS
```

Or ask naturally:

```text
что я говорил про AI Life OS
```

The bot will search memory by embeddings and answer from relevant saved context.

## Daily Review

Start with:

```text
/review
```

The bot asks:

```text
1. Что сделал?
2. Что слил?
3. Что помогло?
4. Что завтра обязательно?
```

Reply in one message. The bot will:

- summarize the review;
- save it to `daily_reviews`;
- save the summary to memory as `reflection`;
- extract patterns.

## Commands

- `/start`: start bot.
- `/help`: list commands.
- `/capture`: prompt for memory capture.
- `/schedule`: show today calendar events.
- `/replan`: explicitly request day replanning.
- `/today`: show adaptive daily direction.
- `/review`: start daily review.
- `/weekly`: build weekly review.
- `/patterns`: show active behavioral patterns.
- `/search`: search memory.
- `/settings`: show current settings guidance.

## Practical Daily Flow

Morning:

```text
что сегодня?
```

If late:

```text
я проспал, сейчас 11:40, перестрой день
```

During day:

```text
идея: ...
```

```text
напомни что я думал про ...
```

Evening:

```text
/review
```

## Troubleshooting

`Календарь не настроен.`

Google Calendar OAuth is not configured or your account is not connected. Set `GOOGLE_CREDENTIALS_FILE` plus `GOOGLE_OAUTH_REDIRECT_URL`, then send `/connect_calendar`.

`Не распознал голос. Повтори текстом.`

Whisper request failed or audio could not be processed. Check `OPENAI_API_KEY` and network access.

`Не сохранил: ошибка памяти.`

PostgreSQL is unavailable or migrations did not run. Start Docker Compose and check `DATABASE_URL`.

`Не разобрал намерение.`

The LLM intent parser failed. Send a shorter, more explicit message.
