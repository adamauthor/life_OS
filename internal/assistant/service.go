package assistant

import (
	"context"
	"fmt"
	"strings"
	"time"

	"life_os/internal/domain"
)

type Service struct {
	AI          AIClient
	Calendar    CalendarService
	Knowledge   *KnowledgeService
	UserProfile *UserProfileService
	Anchors     *AnchorService
	Planning    PlanningService
	Memory      MemoryService
}

func (s *Service) NeedsOnboarding(ctx context.Context, userID domain.UUID) bool {
	return s.UserProfile.IsEmpty(ctx, userID)
}

func OnboardingText() string {
	return strings.Join([]string{
		"Я работаю voice-first: говори или пиши обычным языком.",
		"",
		"Чтобы рекомендации были персональными, ответь одним сообщением:",
		"1. Как тебя называть?",
		"2. Где живешь и какой timezone использовать?",
		"3. Главные цели на год?",
		"4. Что сейчас ломает жизнь?",
		"5. Что помогает вернуться в норму?",
		"6. Какие активности нравятся?",
		"7. Что точно не предлагать?",
		"8. Как выглядит рабочий день?",
		"9. Какие якоря дня подходят и какие бесят?",
		"",
		"Можно коротко, списком. Я сохраню это в профиль.",
	}, "\n")
}

func (s *Service) HandleMessage(ctx context.Context, input AssistantInput) (*AssistantResponse, error) {
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	facts, _ := s.UserProfile.ListFacts(ctx, input.UserID)
	anchors, _ := s.Anchors.List(ctx, input.UserID)

	parsed := routeFallback(input.Text, now)
	if s.AI != nil {
		aiParsed, err := s.AI.ParseAssistantIntent(ctx, IntentInput{
			Text:     input.Text,
			Now:      now,
			Timezone: input.Timezone,
			Facts:    facts,
			Anchors:  anchors,
		})
		if err == nil && aiParsed.Intent.Valid() && aiParsed.Intent != IntentUnknown {
			parsed = mergeParsedIntent(aiParsed, parsed)
		}
	}
	if parsed.RequiresClarification {
		return &AssistantResponse{Text: fallbackClarification(parsed.ClarificationQuestion)}, nil
	}

	switch parsed.Intent {
	case IntentCalendarQuery:
		return s.handleCalendarQuery(ctx, input, parsed)
	case IntentCalendarCreate:
		return s.handleCalendarCreate(ctx, input, parsed)
	case IntentCalendarUpdate:
		return s.handleReplan(ctx, input)
	case IntentTodayDirection:
		return s.handleToday(ctx, input)
	case IntentReplanDay:
		return s.handleReplan(ctx, input)
	case IntentKnowledgeSave:
		return s.handleKnowledgeSave(ctx, input, parsed)
	case IntentKnowledgeQuery:
		return s.handleKnowledgeQuery(ctx, input, parsed)
	case IntentUserProfileUpdate:
		return s.handleProfileUpdate(ctx, input, parsed)
	case IntentUserProfileQuestion:
		return s.handleProfileQuestion(ctx, input)
	case IntentAnchorFeedback:
		return s.handleAnchorFeedback(ctx, input, parsed)
	case IntentDailyReview:
		return &AssistantResponse{Text: "Короткое ревью дня:\n1. Что сделал?\n2. Что слил?\n3. Что помогло?\n4. Что завтра обязательно?"}, nil
	default:
		if s.Memory != nil {
			answer, err := s.Memory.AnswerQuestion(ctx, input.UserID, input.Text)
			if err == nil && strings.TrimSpace(answer) != "" {
				return &AssistantResponse{Text: answer}, nil
			}
		}
		return &AssistantResponse{Text: "Не понял. Скажи проще: календарь, память, долг, цель или что делать сегодня."}, nil
	}
}

func mergeParsedIntent(aiParsed ParsedIntent, fallback ParsedIntent) ParsedIntent {
	if aiParsed.Calendar == nil {
		aiParsed.Calendar = fallback.Calendar
	} else if fallback.Calendar != nil {
		if aiParsed.Calendar.StartTime == "" {
			aiParsed.Calendar.StartTime = fallback.Calendar.StartTime
		}
		if aiParsed.Calendar.EndTime == "" {
			aiParsed.Calendar.EndTime = fallback.Calendar.EndTime
		}
		if aiParsed.Calendar.Title == "" {
			aiParsed.Calendar.Title = fallback.Calendar.Title
		}
		if aiParsed.Calendar.DurationMinutes == 0 {
			aiParsed.Calendar.DurationMinutes = fallback.Calendar.DurationMinutes
		}
	}
	if aiParsed.Knowledge == nil {
		aiParsed.Knowledge = fallback.Knowledge
	} else if fallback.Knowledge != nil {
		if aiParsed.Knowledge.Amount == nil {
			aiParsed.Knowledge.Amount = fallback.Knowledge.Amount
		}
		if aiParsed.Knowledge.Currency == "" {
			aiParsed.Knowledge.Currency = fallback.Knowledge.Currency
		}
		if aiParsed.Knowledge.DueDate == "" {
			aiParsed.Knowledge.DueDate = fallback.Knowledge.DueDate
		}
	}
	if aiParsed.ProfileUpdate == nil {
		aiParsed.ProfileUpdate = fallback.ProfileUpdate
	}
	if aiParsed.AnchorFeedback == nil {
		aiParsed.AnchorFeedback = fallback.AnchorFeedback
	}
	if aiParsed.Query == nil {
		aiParsed.Query = fallback.Query
	}
	return aiParsed
}

func fallbackClarification(question string) string {
	question = strings.TrimSpace(question)
	if question == "" {
		return "Уточни одним сообщением: что сделать и когда."
	}
	return question
}

func (s *Service) handleCalendarQuery(ctx context.Context, input AssistantInput, parsed ParsedIntent) (*AssistantResponse, error) {
	if s.Calendar == nil {
		return &AssistantResponse{Text: "Календарь не подключен. Подключи: /connect_calendar."}, nil
	}
	day := input.Now
	queryRange := "today"
	if parsed.Query != nil && parsed.Query.Range != "" {
		queryRange = parsed.Query.Range
	}
	if queryRange == "tomorrow" {
		day = day.AddDate(0, 0, 1)
	}
	events, err := s.Calendar.ListDayForUser(ctx, input.UserID, day)
	if err != nil {
		return &AssistantResponse{Text: "Не прочитал календарь. Если не подключал Google Calendar, нажми /connect_calendar."}, nil
	}
	if len(events) == 0 {
		return &AssistantResponse{Text: calendarEmptyText(queryRange)}, nil
	}
	lines := []string{calendarHeader(queryRange)}
	for _, event := range events {
		lines = append(lines, fmt.Sprintf("- %s: %s - %s", event.Title, event.Start, event.End))
	}
	return &AssistantResponse{Text: strings.Join(lines, "\n")}, nil
}

func (s *Service) handleCalendarCreate(ctx context.Context, input AssistantInput, parsed ParsedIntent) (*AssistantResponse, error) {
	if s.Calendar == nil {
		return &AssistantResponse{Text: "Календарь не настроен."}, nil
	}
	if parsed.Calendar == nil {
		return &AssistantResponse{Text: "Во сколько поставить?"}, nil
	}
	calendar := parsed.Calendar
	if strings.TrimSpace(calendar.StartTime) == "" {
		return &AssistantResponse{Text: "Во сколько поставить?"}, nil
	}
	duration := calendar.DurationMinutes
	if duration <= 0 {
		duration = 60
	}
	parsedIntent := domain.ParsedIntent{
		Intent:               domain.IntentCreateCalendarEvent,
		Type:                 domain.MemoryTypeEvent,
		Title:                calendar.Title,
		RawText:              input.Text,
		Summary:              input.Text,
		Datetime:             calendar.StartTime,
		DurationMinutes:      duration,
		Tags:                 calendar.Tags,
		Confidence:           parsed.Confidence,
		RequiresConfirmation: true,
	}
	if parsedIntent.Title == "" {
		parsedIntent.Title = input.Text
	}
	action, err := s.Calendar.ProposeEvent(ctx, input.UserID, parsedIntent)
	if err != nil {
		return &AssistantResponse{Text: "Не понял дату/время. Напиши так: завтра в 15:00 созвон."}, nil
	}
	text := fmt.Sprintf("Поставить в календарь?\n\n%s\n%s\n%d минут", parsedIntent.Title, parsedIntent.Datetime, duration)
	return &AssistantResponse{
		Text: text,
		Buttons: []AssistantButton{
			{Text: "Да", Data: fmt.Sprintf("calendar:confirm:%d", action.ID)},
			{Text: "Изменить", Data: fmt.Sprintf("calendar:edit:%d", action.ID)},
			{Text: "Нет", Data: fmt.Sprintf("calendar:cancel:%d", action.ID)},
		},
	}, nil
}

func (s *Service) handleToday(ctx context.Context, input AssistantInput) (*AssistantResponse, error) {
	if s.Planning == nil {
		return s.handleCalendarQuery(ctx, input, ParsedIntent{Query: &QueryIntent{Range: "today"}})
	}
	direction, err := s.Planning.BuildDailyDirection(ctx, input.UserID, input.Now)
	if err != nil {
		return &AssistantResponse{Text: "Не собрал план дня."}, nil
	}
	lines := []string{"Сегодня:"}
	for _, priority := range direction.Priorities {
		if len(lines) >= 6 {
			break
		}
		lines = append(lines, "- "+priority.Title)
	}
	if len(lines) == 1 {
		for _, anchor := range direction.Anchors {
			if len(lines) >= 6 {
				break
			}
			lines = append(lines, "- "+anchor.Title)
		}
	}
	anchors, _ := s.Anchors.List(ctx, input.UserID)
	for _, anchor := range anchors {
		if anchor.Status == "disliked" || anchor.Status == "banned" {
			lines = append(lines, "", "Не ставлю "+anchor.Title+": "+anchor.Reason)
			break
		}
	}
	return &AssistantResponse{Text: strings.Join(lines, "\n")}, nil
}

func (s *Service) handleReplan(ctx context.Context, input AssistantInput) (*AssistantResponse, error) {
	if s.Planning == nil {
		return &AssistantResponse{Text: "Replan пока не настроен."}, nil
	}
	proposal, err := s.Planning.BuildReplanProposal(ctx, input.UserID, input.Text, input.Now)
	if err != nil {
		return &AssistantResponse{Text: "Не смог перестроить день."}, nil
	}
	text := proposal.AuthorityMessage
	if strings.TrimSpace(text) == "" {
		text = proposal.Reason
	}
	if strings.TrimSpace(text) == "" {
		text = "План перестроен. Проверь перед применением."
	}
	return &AssistantResponse{
		Text: text + "\n\nПрименить изменения в календаре?",
		Buttons: []AssistantButton{
			{Text: "Применить", Data: fmt.Sprintf("replan_confirm:%s", proposal.ID.String())},
			{Text: "Изменить", Data: fmt.Sprintf("replan_edit:%s", proposal.ID.String())},
			{Text: "Отклонить", Data: fmt.Sprintf("replan_cancel:%s", proposal.ID.String())},
		},
	}, nil
}

func (s *Service) handleKnowledgeSave(ctx context.Context, input AssistantInput, parsed ParsedIntent) (*AssistantResponse, error) {
	if parsed.Knowledge == nil {
		return &AssistantResponse{Text: "Что именно запомнить?"}, nil
	}
	item, err := s.Knowledge.Save(ctx, input.UserID, input.Text, *parsed.Knowledge)
	if err != nil {
		return &AssistantResponse{Text: "Не сохранил знание."}, nil
	}
	if item.Type == "debt" {
		amount := ""
		if item.Amount != nil {
			amount = formatAmount(*item.Amount, item.Currency)
		}
		return &AssistantResponse{Text: strings.TrimSpace("Запомнил долг:\n\n" + item.Title + " — " + amount + "\n\nСрок: " + formatDueDate(item.DueDate))}, nil
	}
	return &AssistantResponse{Text: "Запомнил: " + item.Title}, nil
}

func (s *Service) handleKnowledgeQuery(ctx context.Context, input AssistantInput, parsed ParsedIntent) (*AssistantResponse, error) {
	queryType := ""
	if parsed.Query != nil {
		queryType = parsed.Query.Type
	}
	if queryType == "debt" || looksLikeDebtQuery(strings.ToLower(input.Text)) {
		debts, err := s.Knowledge.ActiveDebts(ctx, input.UserID)
		if err != nil {
			return &AssistantResponse{Text: "Не прочитал долги."}, nil
		}
		if len(debts) == 0 {
			return &AssistantResponse{Text: "Активных долгов не вижу."}, nil
		}
		lines := []string{"Сейчас активные долги:"}
		for i, debt := range debts {
			lines = append(lines, fmt.Sprintf("%d. %s — %s", i+1, debt.Title, amountLine(debt)))
			if debt.DueDate != nil {
				lines = append(lines, "   Вернуть: "+debt.DueDate.Format("2006-01-02"))
			}
			lines = append(lines, "   Статус: "+debt.Status)
		}
		return &AssistantResponse{Text: strings.Join(lines, "\n")}, nil
	}
	items, err := s.Knowledge.Search(ctx, input.UserID, input.Text)
	if err == nil && len(items) > 0 {
		lines := []string{"Нашел в знаниях:"}
		for _, item := range items {
			lines = append(lines, "- "+item.Title+": "+item.Summary)
		}
		return &AssistantResponse{Text: strings.Join(lines, "\n")}, nil
	}
	if s.Memory != nil {
		answer, err := s.Memory.AnswerQuestion(ctx, input.UserID, input.Text)
		if err == nil {
			return &AssistantResponse{Text: answer}, nil
		}
	}
	return &AssistantResponse{Text: "Не нашел в памяти."}, nil
}

func (s *Service) handleProfileUpdate(ctx context.Context, input AssistantInput, parsed ParsedIntent) (*AssistantResponse, error) {
	if parsed.ProfileUpdate == nil || len(parsed.ProfileUpdate.Facts) == 0 {
		return &AssistantResponse{Text: "Что именно обновить о тебе?"}, nil
	}
	facts, err := s.UserProfile.UpsertFacts(ctx, input.UserID, parsed.ProfileUpdate.Facts)
	if err != nil {
		return &AssistantResponse{Text: "Не сохранил профиль."}, nil
	}
	lines := []string{"Принял. Буду учитывать:"}
	for _, fact := range facts {
		lines = append(lines, "- "+fact.Value)
	}
	return &AssistantResponse{Text: strings.Join(lines, "\n")}, nil
}

func (s *Service) handleProfileQuestion(ctx context.Context, input AssistantInput) (*AssistantResponse, error) {
	facts, err := s.UserProfile.ListFacts(ctx, input.UserID)
	if err != nil || len(facts) == 0 {
		return &AssistantResponse{Text: "Пока мало данных о тебе. Расскажи цели, ограничения и что тебе помогает."}, nil
	}
	lines := []string{"Вот что я знаю:"}
	for _, fact := range facts {
		lines = append(lines, "- "+fact.Category+"."+fact.Key+": "+fact.Value)
	}
	return &AssistantResponse{Text: strings.Join(lines, "\n")}, nil
}

func (s *Service) handleAnchorFeedback(ctx context.Context, input AssistantInput, parsed ParsedIntent) (*AssistantResponse, error) {
	if parsed.AnchorFeedback == nil || len(parsed.AnchorFeedback.Updates) == 0 {
		return &AssistantResponse{Text: "Какой якорь поменять?"}, nil
	}
	updated, err := s.Anchors.ApplyFeedback(ctx, input.UserID, parsed.AnchorFeedback.Updates)
	if err != nil {
		return &AssistantResponse{Text: "Не обновил якоря."}, nil
	}
	lines := []string{"Принял. Обновил якоря:"}
	for _, anchor := range updated {
		lines = append(lines, "- "+anchor.Title+": "+anchor.Status)
	}
	return &AssistantResponse{Text: strings.Join(lines, "\n")}, nil
}

func calendarHeader(queryRange string) string {
	switch queryRange {
	case "tomorrow":
		return "Завтра:"
	case "week":
		return "Ближайшие планы:"
	default:
		return "Сегодня:"
	}
}

func calendarEmptyText(queryRange string) string {
	switch queryRange {
	case "tomorrow":
		return "Завтра событий нет."
	default:
		return "На сегодня событий нет."
	}
}

func formatAmount(amount float64, currency string) string {
	value := fmt.Sprintf("%.0f", amount)
	if currency != "" {
		return value + " " + currency
	}
	return value
}

func amountLine(item domain.KnowledgeItem) string {
	if item.Amount == nil {
		return item.Summary
	}
	return formatAmount(*item.Amount, item.Currency)
}

func formatDueDate(date *time.Time) string {
	if date == nil {
		return "не указан. Добавить срок возврата?"
	}
	return date.Format("2006-01-02")
}
