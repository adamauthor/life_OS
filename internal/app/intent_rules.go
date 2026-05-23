package app

import (
	"regexp"
	"strings"

	"life_os/internal/domain"
)

var timeLikePattern = regexp.MustCompile(`(?i)(^|\s)([01]?\d|2[0-3])([:.][0-5]\d)?($|\s)`)

func normalizeParsedIntent(text string, parsed domain.ParsedIntent) domain.ParsedIntent {
	normalized := strings.ToLower(strings.TrimSpace(text))

	switch {
	case looksLikeReplan(normalized):
		parsed.Intent = domain.IntentReplanDay
		parsed.Type = ""
		parsed.RequiresConfirmation = true
	case looksLikeWeeklyReview(normalized):
		parsed.Intent = domain.IntentWeeklyReview
		parsed.Type = ""
	case looksLikeDailyReview(normalized):
		parsed.Intent = domain.IntentDailyReview
		parsed.Type = domain.MemoryTypeReflection
	case looksLikeMemoryQuestion(normalized):
		parsed.Intent = domain.IntentAskMemory
		parsed.Type = domain.MemoryTypeQuestion
	case looksLikeCalendarEvent(normalized):
		parsed.Intent = domain.IntentCreateCalendarEvent
		parsed.Type = domain.MemoryTypeEvent
		parsed.RequiresConfirmation = true
	case parsed.Intent == domain.IntentCreateTask:
		parsed.Type = domain.MemoryTypeTask
	case parsed.Intent == domain.IntentCaptureMemory && parsed.Type == "":
		parsed.Type = domain.MemoryTypeNote
	case parsed.Intent == "":
		parsed.Intent = domain.IntentUnknown
	}

	return parsed
}

func shouldCaptureAsMemory(intent domain.Intent) bool {
	switch intent {
	case domain.IntentCaptureMemory, domain.IntentCreateTask, domain.IntentHabitLog:
		return true
	default:
		return false
	}
}

func looksLikeReplan(text string) bool {
	return strings.Contains(text, "перестрой") ||
		strings.Contains(text, "перепланируй") ||
		strings.Contains(text, "перенеси день") ||
		strings.Contains(text, "replan") ||
		(strings.Contains(text, "проспал") && strings.Contains(text, "день"))
}

func looksLikeMemoryQuestion(text string) bool {
	return strings.Contains(text, "что я говорил") ||
		strings.Contains(text, "что я писал") ||
		strings.Contains(text, "что я думал") ||
		strings.Contains(text, "напомни что") ||
		strings.Contains(text, "найди в памяти") ||
		strings.Contains(text, "поиск по памяти")
}

func looksLikeDailyReview(text string) bool {
	return strings.Contains(text, "ревью дня") ||
		strings.Contains(text, "итоги дня") ||
		strings.Contains(text, "daily review") ||
		(strings.Contains(text, "что сделал") && strings.Contains(text, "что слил")) ||
		(strings.Contains(text, "сделал") && strings.Contains(text, "помогло") && strings.Contains(text, "завтра"))
}

func looksLikeWeeklyReview(text string) bool {
	return strings.Contains(text, "weekly review") ||
		strings.Contains(text, "недельное ревью") ||
		strings.Contains(text, "ревью недели") ||
		strings.Contains(text, "итоги недели") ||
		strings.Contains(text, "последние 7 дней")
}

func looksLikeCalendarEvent(text string) bool {
	hasCalendarVerb := strings.Contains(text, "добавь") ||
		strings.Contains(text, "поставь") ||
		strings.Contains(text, "запланируй") ||
		strings.Contains(text, "создай событие") ||
		strings.Contains(text, "в календар") ||
		strings.Contains(text, "назначь")

	hasDateWord := strings.Contains(text, "сегодня") ||
		strings.Contains(text, "завтра") ||
		strings.Contains(text, "послезавтра") ||
		strings.Contains(text, "понедельник") ||
		strings.Contains(text, "вторник") ||
		strings.Contains(text, "среду") ||
		strings.Contains(text, "четверг") ||
		strings.Contains(text, "пятницу") ||
		strings.Contains(text, "субботу") ||
		strings.Contains(text, "воскресенье")

	return (hasCalendarVerb && (hasDateWord || timeLikePattern.MatchString(text))) ||
		(hasDateWord && timeLikePattern.MatchString(text))
}
