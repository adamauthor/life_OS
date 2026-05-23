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

BotFather will return `TELEGRAM_BOT_TOKEN`. Put it into `.env` locally or Fly secrets in production.

## Commands

Send:

```text
/setcommands
```

Choose the bot, then paste:

```text
start - Start the assistant
help - Show commands and examples
today - Build adaptive daily direction
replan - Rebuild the day with confirmation
review - Start daily review
weekly - Review the last 7 days
patterns - Show active behavioral patterns
search - Search personal memory
schedule - Show calendar events
capture - Save a thought or note
settings - Show current settings guidance
```

## Description

Send:

```text
/setdescription
```

Choose the bot, then paste:

```text
Adaptive Life OS is a Telegram-first companion for memory, reflection, daily direction, and adaptive replanning.

Send text or voice. The bot classifies intent, saves useful memories, extracts behavioral patterns, and proposes realistic plans.

Calendar changes are never applied automatically. You must confirm them with a button.
```

## About Text

Send:

```text
/setabouttext
```

Choose the bot, then paste:

```text
Memory, reflection, daily direction, and adaptive replanning. Calendar writes only after confirmation.
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

That keeps group behavior limited. For this product, do not promote group usage until permissions and data boundaries are reviewed.

## Inline Mode

Do not enable inline mode for now. The bot does not have an inline UX and memory data should stay private.

## Public Launch Copy

Short announcement:

```text
Adaptive Life OS is a Telegram companion for memory, reviews, behavioral patterns, and realistic daily replanning.

Send text or voice. It can propose calendar actions, but it never changes your calendar without confirmation.
```

