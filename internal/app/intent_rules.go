package app

import (
	"regexp"
	"strings"
	"time"

	"life_os/internal/domain"
)

var timeLikePattern = regexp.MustCompile(`(?i)(^|\s)([01]?\d|2[0-3])([:.][0-5]\d)?($|\s)`)
var prepositionTimePattern = regexp.MustCompile(`(?i)(?:^|\s)(?:в|на)\s*([01]?\d|2[0-3])(?::([0-5]\d))?(?:\s|$)`)
var colonTimePattern = regexp.MustCompile(`(?i)(?:^|\s)([01]?\d|2[0-3]):([0-5]\d)(?:\s|$)`)
var bareTimePattern = regexp.MustCompile(`(?i)(?:^|\s)([01]?\d|2[0-3])(?:\s|$)`)

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

func completeCalendarIntentFromText(text string, parsed domain.ParsedIntent, now time.Time) domain.ParsedIntent {
	if parsed.Intent != domain.IntentCreateCalendarEvent {
		return parsed
	}
	if parsed.DurationMinutes <= 0 {
		parsed.DurationMinutes = 60
	}
	if parsed.Type == "" {
		parsed.Type = domain.MemoryTypeEvent
	}
	parsed.RequiresConfirmation = true

	if strings.TrimSpace(parsed.Datetime) != "" {
		if _, err := time.Parse(time.RFC3339, parsed.Datetime); err == nil {
			return fillCalendarTitle(text, parsed)
		}
	}

	localTime, ok := inferCalendarDateTime(text, now)
	if !ok {
		return fillCalendarTitle(text, parsed)
	}
	parsed.Datetime = localTime.Format(time.RFC3339)
	return fillCalendarTitle(text, parsed)
}

func fillCalendarTitle(text string, parsed domain.ParsedIntent) domain.ParsedIntent {
	if strings.TrimSpace(parsed.Title) != "" {
		return parsed
	}
	title := strings.TrimSpace(text)
	lower := strings.ToLower(title)
	replacements := []string{
		"добавь", "поставь", "запланируй", "создай событие", "создай", "назначь",
		"в календарь", "в календаре", "сегодня", "завтра", "послезавтра",
	}
	for _, word := range replacements {
		lower = strings.ReplaceAll(lower, word, " ")
	}
	lower = regexp.MustCompile(`(?i)(^|\s)(в|на)?\s*([01]?\d|2[0-3])(:[0-5]\d)?($|\s)`).ReplaceAllString(lower, " ")
	lower = strings.Join(strings.Fields(lower), " ")
	if lower == "" {
		return parsed
	}
	parsed.Title = lower
	return parsed
}

func inferCalendarDateTime(text string, now time.Time) (time.Time, bool) {
	normalized := strings.ToLower(strings.TrimSpace(text))
	hasRelativeDay := strings.Contains(normalized, "сегодня") ||
		strings.Contains(normalized, "завтра") ||
		strings.Contains(normalized, "послезавтра")
	if !hasRelativeDay && containsMonthWord(normalized) {
		return time.Time{}, false
	}

	match := prepositionTimePattern.FindStringSubmatch(normalized)
	if len(match) < 2 {
		match = colonTimePattern.FindStringSubmatch(normalized)
	}
	if len(match) < 2 && hasRelativeDay {
		match = bareTimePattern.FindStringSubmatch(normalized)
	}
	if len(match) < 2 {
		return time.Time{}, false
	}
	hour, ok := parseSmallInt(match[1])
	if !ok {
		return time.Time{}, false
	}
	minute := 0
	if len(match) > 2 && match[2] != "" {
		var minuteOK bool
		minute, minuteOK = parseSmallInt(match[2])
		if !minuteOK {
			return time.Time{}, false
		}
	}

	offsetDays := 0
	switch {
	case strings.Contains(normalized, "послезавтра"):
		offsetDays = 2
	case strings.Contains(normalized, "завтра"):
		offsetDays = 1
	case strings.Contains(normalized, "сегодня"):
		offsetDays = 0
	default:
		candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if candidate.After(now) {
			return candidate, true
		}
		offsetDays = 1
	}

	day := now.AddDate(0, 0, offsetDays)
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, now.Location()), true
}

func containsMonthWord(text string) bool {
	months := []string{"январ", "феврал", "март", "апрел", "мая", "май", "июн", "июл", "август", "сентябр", "октябр", "ноябр", "декабр"}
	for _, month := range months {
		if strings.Contains(text, month) {
			return true
		}
	}
	return false
}

func parseSmallInt(value string) (int, bool) {
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
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

func calendarEventClarification(parsed domain.ParsedIntent) string {
	if strings.TrimSpace(parsed.Title) == "" {
		return "Похоже на событие календаря, но нет названия. Напиши так: завтра в 11 разобрать Kafka consumer groups."
	}
	if strings.TrimSpace(parsed.Datetime) == "" {
		return "Похоже на событие календаря, но не хватает даты или времени. Напиши так: завтра в 11 разобрать Kafka consumer groups."
	}
	if _, err := time.Parse(time.RFC3339, parsed.Datetime); err != nil {
		return "Не понял дату/время события. Напиши явно: 24 мая в 11:00 разобрать Kafka consumer groups."
	}
	return ""
}
