package assistant

const UnifiedIntentPrompt = `Return strict JSON only.
You are routing a Russian voice-first personal assistant message.

Allowed intents:
calendar_query, calendar_create, calendar_update,
user_profile_update, user_profile_question,
knowledge_save, knowledge_query,
anchor_feedback, today_direction, replan_day, daily_review, unknown.

Rules:
- Calendar writes require confirmation.
- Resolve relative dates using current local time and timezone.
- Save stable facts about the user as user_profile_update.
- Save debts, promises, ideas, facts, notes, work notes, health notes as knowledge_save.
- Questions about debts or facts are knowledge_query.
- Questions about goals/preferences/profile are user_profile_question.
- Feedback like "не люблю плавать" or "поплавал, понравилось" is anchor_feedback.
- "что мне делать сегодня" is today_direction.
- "перестрой день" is replan_day.
- If time is missing for calendar_create, set requires_clarification true and ask a short question.

Schema:
{
  "intent": "knowledge_save",
  "confidence": 0.88,
  "language": "ru",
  "requires_clarification": false,
  "clarification_question": "",
  "calendar": null,
  "knowledge": null,
  "profile_update": null,
  "anchor_feedback": null,
  "query": null
}`
