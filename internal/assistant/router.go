package assistant

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var amountPattern = regexp.MustCompile(`(?i)(\d+(?:[,.]\d+)?)\s*(млн|миллион|миллиона|миллионов|тыс|тысяч)?`)
var calendarTimePattern = regexp.MustCompile(`(?i)(?:^|\s)(?:в|на)?\s*([01]?\d|2[0-3])(?::([0-5]\d))?(?:\s|$)`)

func routeFallback(text string, now time.Time) ParsedIntent {
	normalized := strings.ToLower(strings.TrimSpace(text))
	switch {
	case looksLikeCalendarQuery(normalized):
		return ParsedIntent{Intent: IntentCalendarQuery, Confidence: 0.7, Query: &QueryIntent{Text: text, Range: calendarRange(normalized)}}
	case looksLikeCalendarUpdate(normalized):
		return ParsedIntent{Intent: IntentCalendarUpdate, Confidence: 0.65, Query: &QueryIntent{Text: text}}
	case looksLikeToday(normalized):
		return ParsedIntent{Intent: IntentTodayDirection, Confidence: 0.7, Query: &QueryIntent{Text: text, Range: "today"}}
	case looksLikeReplan(normalized):
		return ParsedIntent{Intent: IntentReplanDay, Confidence: 0.7, Query: &QueryIntent{Text: text}}
	case looksLikeDebtQuery(normalized):
		return ParsedIntent{Intent: IntentKnowledgeQuery, Confidence: 0.8, Query: &QueryIntent{Text: text, Type: "debt"}}
	case looksLikeDebtSave(normalized):
		return parseDebtFallback(text, now)
	case looksLikeAnchorFeedback(normalized):
		return parseAnchorFallback(text)
	case looksLikeProfileQuestion(normalized):
		return ParsedIntent{Intent: IntentUserProfileQuestion, Confidence: 0.65, Query: &QueryIntent{Text: text}}
	case looksLikeProfileUpdate(normalized):
		return parseProfileFallback(text)
	case looksLikeCalendarCreate(normalized):
		return parseCalendarCreateFallback(text, now)
	case looksLikeKnowledgeQuery(normalized):
		return ParsedIntent{Intent: IntentKnowledgeQuery, Confidence: 0.55, Query: &QueryIntent{Text: text}}
	default:
		return ParsedIntent{Intent: IntentKnowledgeSave, Confidence: 0.4, Knowledge: &KnowledgeIntent{Type: "general_note", Title: firstWords(text, 8), Summary: text, Tags: []string{"note"}}}
	}
}

func parseCalendarCreateFallback(text string, now time.Time) ParsedIntent {
	start, ok := inferCalendarStart(text, now)
	calendar := &CalendarIntent{
		Title:                cleanCalendarTitle(text),
		DurationMinutes:      60,
		RequiresConfirmation: true,
	}
	if ok {
		calendar.StartTime = start.Format(time.RFC3339)
		calendar.EndTime = start.Add(time.Hour).Format(time.RFC3339)
	} else {
		return ParsedIntent{
			Intent:                IntentCalendarCreate,
			Confidence:            0.55,
			RequiresClarification: true,
			ClarificationQuestion: "Во сколько поставить?",
			Calendar:              calendar,
		}
	}
	return ParsedIntent{Intent: IntentCalendarCreate, Confidence: 0.65, Calendar: calendar}
}

func inferCalendarStart(text string, now time.Time) (time.Time, bool) {
	normalized := strings.ToLower(text)
	hasDate := strings.Contains(normalized, "сегодня") || strings.Contains(normalized, "завтра") || strings.Contains(normalized, "послезавтра")
	match := calendarTimePattern.FindStringSubmatch(normalized)
	hour := 15
	minute := 0
	if len(match) >= 2 {
		parsedHour, err := strconv.Atoi(match[1])
		if err == nil {
			hour = parsedHour
		}
		if len(match) >= 3 && match[2] != "" {
			parsedMinute, err := strconv.Atoi(match[2])
			if err == nil {
				minute = parsedMinute
			}
		}
	} else if !hasDate {
		return time.Time{}, false
	}
	offset := 0
	switch {
	case strings.Contains(normalized, "послезавтра"):
		offset = 2
	case strings.Contains(normalized, "завтра"):
		offset = 1
	case strings.Contains(normalized, "сегодня"):
		offset = 0
	default:
		candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if candidate.After(now) {
			return candidate, true
		}
		offset = 1
	}
	day := now.AddDate(0, 0, offset)
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, now.Location()), true
}

func cleanCalendarTitle(text string) string {
	title := strings.ToLower(strings.TrimSpace(text))
	for _, token := range []string{"запиши", "поставь", "добавь", "запланируй", "сегодня", "завтра", "послезавтра", "в календарь"} {
		title = strings.ReplaceAll(title, token, " ")
	}
	title = calendarTimePattern.ReplaceAllString(title, " ")
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		return strings.TrimSpace(text)
	}
	return title
}

func looksLikeCalendarQuery(text string) bool {
	return strings.Contains(text, "что у меня") ||
		strings.Contains(text, "какие планы") ||
		strings.Contains(text, "что по планам") ||
		strings.Contains(text, "расписание")
}

func looksLikeCalendarUpdate(text string) bool {
	return strings.Contains(text, "перенеси") ||
		strings.Contains(text, "сдвинь") ||
		strings.Contains(text, "передвинь") ||
		strings.Contains(text, "переставь")
}

func looksLikeToday(text string) bool {
	return strings.Contains(text, "что мне делать сегодня") ||
		strings.Contains(text, "направление дня") ||
		strings.Contains(text, "план на сегодня")
}

func looksLikeReplan(text string) bool {
	return strings.Contains(text, "перестрой") || strings.Contains(text, "перепланируй") || strings.Contains(text, "проспал")
}

func looksLikeDebtSave(text string) bool {
	return strings.Contains(text, "я должен") || strings.Contains(text, "должен ") || strings.Contains(text, "долг")
}

func looksLikeDebtQuery(text string) bool {
	return strings.Contains(text, "кому я должен") || strings.Contains(text, "мои долги") || strings.Contains(text, "должен деньги")
}

func looksLikeAnchorFeedback(text string) bool {
	return strings.Contains(text, "не люблю") ||
		strings.Contains(text, "не предлагай") ||
		strings.Contains(text, "понравилось") ||
		strings.Contains(text, "можно чаще") ||
		strings.Contains(text, "бесит")
}

func looksLikeProfileUpdate(text string) bool {
	return strings.Contains(text, "моя цель") ||
		strings.Contains(text, "цель на год") ||
		strings.Contains(text, "я хочу") ||
		strings.Contains(text, "мне помогает") ||
		strings.Contains(text, "мне подходят")
}

func looksLikeProfileQuestion(text string) bool {
	return strings.Contains(text, "что я хочу улучшить") ||
		strings.Contains(text, "мои цели") ||
		strings.Contains(text, "какие у меня цели") ||
		strings.Contains(text, "что ты знаешь обо мне")
}

func looksLikeCalendarCreate(text string) bool {
	return strings.Contains(text, "запиши") || strings.Contains(text, "поставь") || strings.Contains(text, "добавь") || strings.Contains(text, "запланируй")
}

func looksLikeKnowledgeQuery(text string) bool {
	return strings.HasPrefix(text, "что я") || strings.Contains(text, "что говорил") || strings.Contains(text, "напомни")
}

func calendarRange(text string) string {
	switch {
	case strings.Contains(text, "завтра"):
		return "tomorrow"
	case strings.Contains(text, "недел"):
		return "week"
	default:
		return "today"
	}
}

func parseDebtFallback(text string, now time.Time) ParsedIntent {
	person := extractDebtPerson(text)
	amount, currency := extractAmountCurrency(text)
	dueDate := ""
	if strings.Contains(strings.ToLower(text), "до конца месяца") {
		dueDate = endOfMonth(now).Format("2006-01-02")
	}
	title := "Долг"
	if person != "" {
		title = "Долг " + person
	}
	return ParsedIntent{
		Intent:     IntentKnowledgeSave,
		Confidence: 0.75,
		Knowledge: &KnowledgeIntent{
			Type:    "debt",
			Title:   title,
			Summary: text,
			Entities: map[string]any{
				"person":    person,
				"direction": "user_owes",
			},
			Amount:   amount,
			Currency: currency,
			DueDate:  dueDate,
			Tags:     []string{"debt", "money"},
		},
	}
}

func extractDebtPerson(text string) string {
	words := strings.Fields(text)
	for i, word := range words {
		if strings.EqualFold(word, "должен") && i+1 < len(words) {
			return strings.Trim(words[i+1], " ,.!?;:")
		}
	}
	return ""
}

func extractAmountCurrency(text string) (*float64, string) {
	match := amountPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return nil, ""
	}
	raw := strings.ReplaceAll(match[1], ",", ".")
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, ""
	}
	if len(match) > 2 {
		switch strings.ToLower(match[2]) {
		case "млн", "миллион", "миллиона", "миллионов":
			value *= 1000000
		case "тыс", "тысяч":
			value *= 1000
		}
	}
	currency := ""
	lower := strings.ToLower(text)
	if strings.Contains(lower, "донг") || strings.Contains(lower, "vnd") {
		currency = "VND"
	}
	return &value, currency
}

func parseAnchorFallback(text string) ParsedIntent {
	normalized := strings.ToLower(text)
	updates := []AnchorUpdateIntent{}
	if strings.Contains(normalized, "плав") || strings.Contains(normalized, "море") {
		status := "disliked"
		score := -0.7
		if strings.Contains(normalized, "понравилось") || strings.Contains(normalized, "можно чаще") {
			status = "liked"
			score = 0.5
		}
		updates = append(updates, AnchorUpdateIntent{AnchorCode: "sea_swim", Title: "Плавание", PreferenceScore: score, Status: status, Reason: text})
	}
	if strings.Contains(normalized, "прогул") || strings.Contains(normalized, "горы") {
		updates = append(updates, AnchorUpdateIntent{AnchorCode: "morning_walk", Title: "Прогулка", PreferenceScore: 0.6, Status: "preferred", Reason: text})
	}
	return ParsedIntent{Intent: IntentAnchorFeedback, Confidence: 0.7, AnchorFeedback: &AnchorFeedbackIntent{Updates: updates}}
}

func parseProfileFallback(text string) ParsedIntent {
	category := "preferences"
	key := stableKey(text)
	if strings.Contains(strings.ToLower(text), "цель") || strings.Contains(strings.ToLower(text), "хочу") {
		category = "goals"
		key = "current_goals"
	}
	return ParsedIntent{Intent: IntentUserProfileUpdate, Confidence: 0.6, ProfileUpdate: &ProfileUpdateIntent{Facts: []ProfileFactIntent{{Category: category, Key: key, Value: text, Confidence: 0.7}}}}
}

func endOfMonth(now time.Time) time.Time {
	firstNext := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	return firstNext.AddDate(0, 0, -1)
}

func firstWords(text string, limit int) string {
	words := strings.Fields(text)
	if len(words) <= limit {
		return text
	}
	return strings.Join(words[:limit], " ")
}
