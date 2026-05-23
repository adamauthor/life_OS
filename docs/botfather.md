# BotFather Setup

Use the official `@BotFather` bot in Telegram.

## Create Bot

Send:

```text
/newbot
```

Name:

```text
Adaptive Life OS
```

Username examples:

```text
adaptive_life_os_bot
life_os_companion_bot
yourname_life_os_bot
```

BotFather returns `TELEGRAM_BOT_TOKEN`. Store it in `.env` locally or Fly secrets in production.

## Commands

Send:

```text
/setcommands
```

Choose the bot, then paste:

```text
start - Start and connect calendar
help - Show commands and examples
today - Build adaptive daily direction
replan - Rebuild the day with confirmation
review - Start daily review
weekly - Review the last 7 days
patterns - Show active behavioral patterns
autonomy - Configure proactive reminders
connect_calendar - Connect Google Calendar
calendar_status - Show calendar connection status
disconnect_calendar - Disconnect Google Calendar
search - Search personal memory
schedule - Show today's calendar events
capture - Save a thought or note
settings - Show settings guidance
```

## Description

Send:

```text
/setdescription
```

Choose the bot, then paste:

```text
Adaptive Life OS is a Telegram-first companion for memory, reflection, behavioral patterns, daily direction, and adaptive replanning.

Send text or voice. The bot saves useful memories, reviews the day, detects patterns, and proposes realistic plans.

Each user can connect their own Google Calendar. Calendar changes are never applied automatically; you confirm them with a button.
```

## About Text

Send:

```text
/setabouttext
```

Choose the bot, then paste:

```text
Memory, reviews, behavioral patterns, daily direction, and replanning. Calendar writes only after confirmation.
```

## Bot Picture

Send:

```text
/setuserpic
```

Recommended avatar brief:

```text
Minimal dark icon: compass + memory node, calm productivity assistant, no cute mascot, no gradient-heavy background.
```

Use a square PNG, ideally 1024x1024.

## Privacy And Groups

This bot is designed for private 1:1 chat.

Recommended:

```text
/setprivacy
```

Choose:

```text
Enable
```

That limits group behavior. Do not promote group usage until permissions, logging, and data boundaries are reviewed.

## Inline Mode

Do not enable inline mode. The bot has no inline UX and memory data should stay private.

## Public Launch Copy

Short announcement:

```text
Adaptive Life OS is a Telegram companion for memory, reviews, behavioral patterns, and realistic daily replanning.

Send text or voice. Connect your own Google Calendar if you want calendar-aware planning. Calendar writes only happen after confirmation.
```
