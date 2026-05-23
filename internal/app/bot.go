package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"life_os/internal/companion"
	"life_os/internal/domain"
	"life_os/internal/patterns"
	"life_os/internal/planning"
	reviewsvc "life_os/internal/review"
	"life_os/internal/telegram"
)

type TelegramClient interface {
	Updates(ctx context.Context) <-chan telegram.Update
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageWithButtons(ctx context.Context, chatID int64, text string, buttons []telegram.InlineButton) error
	AnswerCallback(ctx context.Context, callbackID string, text string) error
	DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, string, error)
}

type Bot struct {
	client         TelegramClient
	logger         *slog.Logger
	memories       *MemoryService
	calendar       *CalendarService
	reviews        *ReviewService
	planning       *planning.Service
	reviewV2       *reviewsvc.Service
	patterns       *patterns.Service
	companion      *companion.Service
	ai             AIClient
	timezone       *time.Location
	pendingReviews map[int64]time.Time
}

func NewBot(client TelegramClient, memories *MemoryService, calendar *CalendarService, reviews *ReviewService, ai AIClient, timezone *time.Location, logger *slog.Logger) *Bot {
	if timezone == nil {
		timezone = time.UTC
	}
	return &Bot{
		client:         client,
		logger:         logger,
		memories:       memories,
		calendar:       calendar,
		reviews:        reviews,
		ai:             ai,
		timezone:       timezone,
		pendingReviews: make(map[int64]time.Time),
	}
}

func (b *Bot) ConfigureAdaptiveServices(planningService *planning.Service, reviewService *reviewsvc.Service, patternService *patterns.Service, companionService *companion.Service) {
	b.planning = planningService
	b.reviewV2 = reviewService
	b.patterns = patternService
	b.companion = companionService
}

func (b *Bot) Run(ctx context.Context) error {
	b.logger.Info("telegram bot started")

	updates := b.client.Updates(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			b.handleUpdate(ctx, update)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update telegram.Update) {
	if update.CallbackQuery != nil {
		b.handleCallback(ctx, update)
		return
	}
	if update.Message == nil {
		return
	}

	msg := update.Message
	b.logger.Info(
		"incoming telegram message",
		"update_id", update.UpdateID,
		"message_id", msg.MessageID,
		"chat_id", chatID(msg),
		"user_id", userID(msg),
		"username", username(msg),
		"text", msg.Text,
	)

	if msg.Voice != nil {
		b.handleVoice(ctx, msg)
		return
	}
	if msg.Text == "" {
		return
	}

	if response := b.routeText(ctx, msg); response != "" {
		if err := b.client.SendMessage(ctx, chatID(msg), response); err != nil {
			b.logger.Error("failed to send telegram message", "error", err, "chat_id", chatID(msg))
		}
		return
	}

	b.handleNaturalText(ctx, msg)
}

func (b *Bot) routeText(ctx context.Context, msg *telegram.Message) string {
	text := msg.Text
	command := strings.Fields(text)
	if len(command) == 0 || !strings.HasPrefix(command[0], "/") {
		return ""
	}

	switch strings.Split(command[0], "@")[0] {
	case "/start":
		return startText()
	case "/help":
		return helpText()
	case "/capture":
		return "Пришли мысль, задачу, идею или заметку одним сообщением."
	case "/schedule":
		return b.schedule(ctx, msg)
	case "/today":
		return b.today(ctx, msg)
	case "/replan":
		b.handleReplan(ctx, msg)
		return "Принял запрос на перепланирование."
	case "/review":
		b.pendingReviews[userID(msg)] = time.Now().In(b.timezone)
		if b.reviewV2 != nil {
			_ = b.reviewV2.StartDailyReview(ctx, domain.UserIDFromTelegram(userID(msg)))
		}
		return dailyReviewQuestions()
	case "/weekly":
		return b.weekly(ctx, msg)
	case "/patterns":
		return b.listPatterns(ctx, msg)
	case "/search":
		query := strings.TrimSpace(strings.TrimPrefix(text, command[0]))
		if query == "" {
			return "Напиши так: /search что я говорил про AI Life OS"
		}
		b.handleMemoryQuestion(ctx, msg, query)
		return "Ищу в памяти."
	case "/settings":
		return "Настройки профиля пока через переменные окружения: APP_TIMEZONE, GOOGLE_CALENDAR_ID."
	default:
		return "Неизвестная команда. Напиши /help."
	}
}

func (b *Bot) handleNaturalText(ctx context.Context, msg *telegram.Message) {
	b.handleNaturalTextSource(ctx, msg, "telegram")
}

func (b *Bot) handleVoice(ctx context.Context, msg *telegram.Message) {
	file, filename, err := b.client.DownloadFile(ctx, msg.Voice.FileID)
	if err != nil {
		b.logger.Error("failed to download voice", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не скачал voice message.")
		return
	}
	defer file.Close()

	transcription, err := b.ai.Transcribe(ctx, filename, file)
	if err != nil {
		b.logger.Error("failed to transcribe voice", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не распознал голос. Повтори текстом.")
		return
	}
	msg.Text = transcription
	b.handleNaturalTextSource(ctx, msg, "telegram_voice")
}

func (b *Bot) handleCalendarProposal(ctx context.Context, msg *telegram.Message, parsed domain.ParsedIntent) {
	if b.calendar == nil {
		_ = b.client.SendMessage(ctx, chatID(msg), "Календарь не настроен.")
		return
	}
	action, err := b.calendar.ProposeEvent(ctx, domain.UserIDFromTelegram(userID(msg)), parsed)
	if err != nil {
		b.logger.Error("failed to propose calendar event", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не смог подготовить событие. Укажи дату и время явно.")
		return
	}
	duration := parsed.DurationMinutes
	if duration <= 0 {
		duration = 60
	}
	text := fmt.Sprintf("Я понял это как событие:\n\nНазвание: %s\nДата/время: %s\nДлительность: %d минут\n\nДобавить в календарь?", parsed.Title, parsed.Datetime, duration)
	if err := b.client.SendMessageWithButtons(ctx, chatID(msg), text, []telegram.InlineButton{
		{Text: "Да", Data: fmt.Sprintf("calendar:confirm:%d", action.ID)},
		{Text: "Изменить", Data: fmt.Sprintf("calendar:edit:%d", action.ID)},
		{Text: "Нет", Data: fmt.Sprintf("calendar:cancel:%d", action.ID)},
	}); err != nil {
		b.logger.Error("failed to send calendar proposal", "error", err)
	}
}

func (b *Bot) handleMemoryQuestion(ctx context.Context, msg *telegram.Message, question string) {
	if b.memories == nil {
		_ = b.client.SendMessage(ctx, chatID(msg), "Память не настроена.")
		return
	}
	answer, err := b.memories.AnswerQuestion(ctx, domain.UserIDFromTelegram(userID(msg)), question)
	if err != nil {
		b.logger.Error("failed to answer memory question", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не смог найти ответ в памяти.")
		return
	}
	_ = b.client.SendMessage(ctx, chatID(msg), answer)
}

func (b *Bot) handleReplan(ctx context.Context, msg *telegram.Message) {
	if b.planning != nil {
		input := commandPayload(msg.Text)
		if input == "" {
			input = msg.Text
		}
		if strings.TrimSpace(input) == "" || strings.HasPrefix(strings.TrimSpace(input), "/replan") {
			input = "Перестрой день исходя из текущего состояния и календаря."
		}
		proposal, err := b.planning.BuildReplanProposal(ctx, domain.UserIDFromTelegram(userID(msg)), input, time.Now().In(b.timezone))
		if err != nil {
			b.logger.Error("failed to build adaptive replan", "error", err)
			_ = b.client.SendMessage(ctx, chatID(msg), "Не смог перестроить день.")
			return
		}
		if b.companion != nil {
			signal, err := b.companion.DetectAvoidanceContext(ctx, domain.UserIDFromTelegram(userID(msg)), input)
			if err == nil && signal != nil && signal.Detected {
				message, err := b.companion.FormatAuthorityResponse(ctx, domain.CompanionResponseInput{
					Message:         proposal.AuthorityMessage,
					NextStep:        firstPlanStep(proposal.ProposedPlan),
					AvoidanceSignal: signal,
				})
				if err == nil {
					proposal.AuthorityMessage = message
				}
			}
		}
		text := formatAdaptiveReplanProposal(*proposal)
		if err := b.client.SendMessageWithButtons(ctx, chatID(msg), text+"\n\nПрименить изменения в календаре?", []telegram.InlineButton{
			{Text: "Применить", Data: fmt.Sprintf("replan_confirm:%s", proposal.ID.String())},
			{Text: "Изменить", Data: fmt.Sprintf("replan_edit:%s", proposal.ID.String())},
			{Text: "Отклонить", Data: fmt.Sprintf("replan_cancel:%s", proposal.ID.String())},
		}); err != nil {
			b.logger.Error("failed to send adaptive replan proposal", "error", err)
		}
		return
	}
	if b.ai == nil {
		_ = b.client.SendMessage(ctx, chatID(msg), "AI client не настроен.")
		return
	}
	if b.calendar == nil {
		_ = b.client.SendMessage(ctx, chatID(msg), "Календарь не настроен.")
		return
	}
	var events []CalendarEvent
	dayEvents, err := b.calendar.ListDay(ctx, time.Now().In(b.timezone))
	if err != nil {
		b.logger.Error("failed to list calendar events for replan", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не смог прочитать календарь для перепланирования.")
		return
	} else {
		events = dayEvents
	}
	proposal, err := b.ai.ReplanDay(ctx, msg.Text, events)
	if err != nil {
		b.logger.Error("failed to replan day", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не смог перестроить день.")
		return
	}
	action, err := b.calendar.ProposeReplan(ctx, domain.UserIDFromTelegram(userID(msg)), proposal)
	if err != nil {
		b.logger.Error("failed to save replan proposal", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не смог сохранить план на подтверждение.")
		return
	}
	text := formatReplanProposal(proposal)
	if err := b.client.SendMessageWithButtons(ctx, chatID(msg), text+"\n\nПрименить изменения в календаре?", []telegram.InlineButton{
		{Text: "Да", Data: fmt.Sprintf("calendar:confirm:%d", action.ID)},
		{Text: "Изменить", Data: fmt.Sprintf("calendar:edit:%d", action.ID)},
		{Text: "Нет", Data: fmt.Sprintf("calendar:cancel:%d", action.ID)},
	}); err != nil {
		b.logger.Error("failed to send replan proposal", "error", err)
	}
}

func (b *Bot) handleDailyReviewText(ctx context.Context, msg *telegram.Message) {
	if b.reviewV2 != nil {
		review, err := b.reviewV2.SaveDailyReview(ctx, domain.UserIDFromTelegram(userID(msg)), msg.Text)
		if err != nil {
			b.logger.Error("failed to save adaptive daily review", "error", err)
			_ = b.client.SendMessage(ctx, chatID(msg), "Не сохранил review.")
			return
		}
		_ = b.client.SendMessage(ctx, chatID(msg), formatDailyReviewSaved(*review))
		return
	}
	if b.reviews == nil {
		_ = b.client.SendMessage(ctx, chatID(msg), "Review storage не настроен.")
		return
	}
	if _, err := b.reviews.SaveDailyReview(ctx, msg.Text, time.Now().In(b.timezone)); err != nil {
		b.logger.Error("failed to save daily review", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не сохранил review.")
		return
	}
	_ = b.client.SendMessage(ctx, chatID(msg), "Review сохранил. Завтра обязательный шаг: выбери один главный результат до начала дня.")
}

func (b *Bot) handleCallback(ctx context.Context, update telegram.Update) {
	callback := update.CallbackQuery
	if callback == nil || callback.Message == nil {
		return
	}
	if strings.HasPrefix(callback.Data, "replan_") {
		b.handleReplanCallback(ctx, callback)
		return
	}
	parts := strings.Split(callback.Data, ":")
	if len(parts) != 3 || parts[0] != "calendar" {
		_ = b.client.AnswerCallback(ctx, callback.ID, "Неизвестное действие")
		return
	}
	if b.calendar == nil {
		_ = b.client.AnswerCallback(ctx, callback.ID, "Календарь не настроен")
		return
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		_ = b.client.AnswerCallback(ctx, callback.ID, "Некорректное действие")
		return
	}

	chatID := callbackChatID(callback)
	switch parts[1] {
	case "confirm":
		eventID, err := b.calendar.ConfirmAction(ctx, domain.UserIDFromTelegram(callbackUserID(callback)), id)
		if err != nil {
			b.logger.Error("failed to confirm calendar action", "error", err, "action_id", id)
			_ = b.client.AnswerCallback(ctx, callback.ID, "Ошибка")
			_ = b.client.SendMessage(ctx, chatID, "Не применил событие: "+err.Error())
			return
		}
		_ = b.client.AnswerCallback(ctx, callback.ID, "Готово")
		_ = b.client.SendMessage(ctx, chatID, "Применил в календаре: "+eventID)
	case "cancel":
		if err := b.calendar.CancelAction(ctx, domain.UserIDFromTelegram(callbackUserID(callback)), id); err != nil {
			b.logger.Error("failed to cancel calendar action", "error", err, "action_id", id)
		}
		_ = b.client.AnswerCallback(ctx, callback.ID, "Отменено")
		_ = b.client.SendMessage(ctx, chatID, "Ок, не добавляю.")
	case "edit":
		_ = b.client.AnswerCallback(ctx, callback.ID, "Изменить")
		_ = b.client.SendMessage(ctx, chatID, "Напиши исправленное событие одним сообщением: что, когда, длительность.")
	}
}

func (b *Bot) handleReplanCallback(ctx context.Context, callback *telegram.CallbackQuery) {
	if b.planning == nil {
		_ = b.client.AnswerCallback(ctx, callback.ID, "Planning не настроен")
		return
	}
	parts := strings.SplitN(callback.Data, ":", 2)
	if len(parts) != 2 {
		_ = b.client.AnswerCallback(ctx, callback.ID, "Некорректное действие")
		return
	}
	proposalID, err := uuid.Parse(parts[1])
	if err != nil {
		_ = b.client.AnswerCallback(ctx, callback.ID, "Некорректный proposal id")
		return
	}
	chatID := callbackChatID(callback)

	switch parts[0] {
	case "replan_confirm":
		if err := b.planning.ConfirmReplanForUser(ctx, domain.UserIDFromTelegram(callbackUserID(callback)), proposalID); err != nil {
			b.logger.Error("failed to confirm replan", "error", err, "proposal_id", proposalID.String())
			_ = b.client.AnswerCallback(ctx, callback.ID, "Ошибка")
			_ = b.client.SendMessage(ctx, chatID, "Не применил replan: "+err.Error())
			return
		}
		_ = b.client.AnswerCallback(ctx, callback.ID, "Применено")
		_ = b.client.SendMessage(ctx, chatID, "Применил подтвержденные изменения в календаре.")
	case "replan_cancel":
		if err := b.planning.CancelReplanForUser(ctx, domain.UserIDFromTelegram(callbackUserID(callback)), proposalID); err != nil {
			b.logger.Error("failed to cancel replan", "error", err, "proposal_id", proposalID.String())
		}
		_ = b.client.AnswerCallback(ctx, callback.ID, "Отклонено")
		_ = b.client.SendMessage(ctx, chatID, "Ок, изменения в календаре не применяю.")
	case "replan_edit":
		_ = b.client.AnswerCallback(ctx, callback.ID, "Изменить")
		_ = b.client.SendMessage(ctx, chatID, "Напиши корректировку одним сообщением: что поменять, какие fixed events не трогать, какой главный блок сохранить.")
	default:
		_ = b.client.AnswerCallback(ctx, callback.ID, "Неизвестное действие")
	}
}

func (b *Bot) schedule(ctx context.Context, msg *telegram.Message) string {
	if b.calendar == nil {
		return "Календарь не настроен."
	}
	events, err := b.calendar.ListDay(ctx, time.Now().In(b.timezone))
	if err != nil {
		b.logger.Error("failed to list today calendar", "error", err)
		return "Не смог прочитать календарь. Проверь GOOGLE_CALENDAR_ID: для основного календаря используй primary."
	}
	if len(events) == 0 {
		return "На сегодня событий нет."
	}
	lines := []string{"Сегодня:"}
	for _, event := range events {
		lines = append(lines, fmt.Sprintf("- %s: %s - %s", event.Title, event.Start, event.End))
	}
	return strings.Join(lines, "\n")
}

func (b *Bot) today(ctx context.Context, msg *telegram.Message) string {
	if b.planning == nil {
		return b.schedule(ctx, msg)
	}
	direction, err := b.planning.BuildDailyDirection(ctx, domain.UserIDFromTelegram(userID(msg)), time.Now().In(b.timezone))
	if err != nil {
		b.logger.Error("failed to build daily direction", "error", err)
		return "Не смог собрать направление дня."
	}
	return formatDailyDirection(*direction)
}

func (b *Bot) weekly(ctx context.Context, msg *telegram.Message) string {
	if b.reviewV2 == nil {
		return "Weekly review не настроен."
	}
	now := time.Now().In(b.timezone)
	weekStart := startOfWeek(now)
	text, err := b.reviewV2.BuildWeeklyReview(ctx, domain.UserIDFromTelegram(userID(msg)), weekStart)
	if err != nil {
		b.logger.Error("failed to build weekly review", "error", err)
		return "Не смог собрать weekly review."
	}
	return text
}

func (b *Bot) listPatterns(ctx context.Context, msg *telegram.Message) string {
	if b.patterns == nil {
		return "Patterns не настроены."
	}
	active, err := b.patterns.ListActive(ctx, domain.UserIDFromTelegram(userID(msg)))
	if err != nil {
		b.logger.Error("failed to list patterns", "error", err)
		return "Не смог прочитать patterns."
	}
	if len(active) == 0 {
		return "Активных behavioral patterns пока нет."
	}
	lines := []string{"Active behavioral patterns:"}
	for i, pattern := range active {
		lines = append(lines, fmt.Sprintf("%d. %s — confidence %.2f", i+1, pattern.Code, pattern.Confidence))
	}
	return strings.Join(lines, "\n")
}

func (b *Bot) captureTextWithParsed(ctx context.Context, msg *telegram.Message, parsed domain.ParsedIntent) error {
	return b.captureTextWithParsedSource(ctx, msg, parsed, "telegram")
}

func (b *Bot) handleNaturalTextSource(ctx context.Context, msg *telegram.Message, source string) {
	if _, ok := b.pendingReviews[userID(msg)]; ok {
		delete(b.pendingReviews, userID(msg))
		b.handleDailyReviewText(ctx, msg)
		return
	}
	if b.ai == nil {
		_ = b.client.SendMessage(ctx, chatID(msg), "AI client не настроен.")
		return
	}
	parsed, err := b.ai.ParseIntent(ctx, msg.Text, b.now(), b.timezone.String())
	if err != nil {
		b.logger.Error("failed to parse intent", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не разобрал намерение. Переформулируй короче.")
		return
	}
	parsed = normalizeParsedIntent(msg.Text, parsed)

	switch parsed.Intent {
	case domain.IntentCreateCalendarEvent:
		b.handleCalendarProposal(ctx, msg, parsed)
	case domain.IntentAskMemory:
		b.handleMemoryQuestion(ctx, msg, msg.Text)
	case domain.IntentReplanDay:
		b.handleReplan(ctx, msg)
	case domain.IntentDailyReview:
		b.handleDailyReviewText(ctx, msg)
	case domain.IntentWeeklyReview:
		_ = b.client.SendMessage(ctx, chatID(msg), b.weekly(ctx, msg))
	default:
		if !shouldCaptureAsMemory(parsed.Intent) {
			_ = b.client.SendMessage(ctx, chatID(msg), "Не уверен, что это нужно сохранять в память. Скажи явно: идея, заметка, задача, событие или вопрос.")
			return
		}
		if b.memories == nil {
			_ = b.client.SendMessage(ctx, chatID(msg), "Память не настроена.")
			return
		}
		if err := b.captureTextWithParsedSource(ctx, msg, parsed, source); err != nil {
			b.logger.Error("failed to capture telegram text", "error", err)
			_ = b.client.SendMessage(ctx, chatID(msg), "Не сохранил: ошибка памяти. Проверь базу и повтори.")
			return
		}
		_ = b.client.SendMessage(ctx, chatID(msg), "Сохранил в память.")
	}
}

func (b *Bot) captureTextWithParsedSource(ctx context.Context, msg *telegram.Message, parsed domain.ParsedIntent, source string) error {
	_, err := b.memories.CaptureParsedTelegramText(ctx, CaptureTelegramTextInput{
		Text:       msg.Text,
		Source:     source,
		ChatID:     chatID(msg),
		MessageID:  msg.MessageID,
		UserID:     userID(msg),
		Username:   username(msg),
		TelegramAt: msg.Date,
		Timezone:   b.timezone.String(),
		NowRFC3339: b.now(),
	}, parsed)
	return err
}

func (b *Bot) now() string {
	return time.Now().In(b.timezone).Format(time.RFC3339)
}

func helpText() string {
	return strings.Join([]string{
		"Adaptive Life Companion",
		"",
		"Можно писать обычным текстом или voice. Команды не обязательны.",
		"",
		"Примеры:",
		"идея: сервис учета калорий как финансовый бюджет",
		"завтра в 11 разобрать Kafka consumer groups",
		"я проспал, сейчас 11:40, перестрой день",
		"что я говорил про AI Life OS",
		"",
		"Команды:",
		"/start - краткий user guide",
		"/help - список команд и примеры",
		"/capture - сохранить мысль, задачу, идею или заметку",
		"/schedule - события календаря",
		"/today - направление дня",
		"/replan - перестроить день",
		"/review - daily review",
		"/weekly - weekly review",
		"/patterns - active behavioral patterns",
		"/search <вопрос> - поиск по памяти",
		"/settings - настройки профиля",
		"",
		"Правило: календарь меняю только после подтверждения кнопкой.",
	}, "\n")
}

func startText() string {
	return strings.Join([]string{
		"Adaptive Life Companion включен.",
		"",
		"Как пользоваться:",
		"1. Пиши или говори естественно. Команды не обязательны.",
		"2. Я сам определю: память, задача, событие, поиск, review или replan.",
		"3. Календарь меняю только после твоего подтверждения.",
		"",
		"Voice-first примеры:",
		"- я проспал, сейчас 11:40, перестрой день",
		"- завтра в 11 разобрать Kafka consumer groups",
		"- идея: сервис учета калорий как финансовый бюджет",
		"",
		"Команды:",
		"/help - полный список",
		"/today - направление дня",
		"/replan - перестроить день",
		"/review - daily review",
		"/weekly - weekly review",
		"/patterns - behavioral patterns",
		"/search <вопрос> - поиск по памяти",
		"/settings - настройки",
		"",
		"Следующий шаг: отправь мысль, задачу, событие или voice.",
	}, "\n")
}

func dailyReviewQuestions() string {
	return strings.Join([]string{
		"Короткое ревью дня:",
		"1. Что сделал?",
		"2. Что слил?",
		"3. Что помогло?",
		"4. Что ухудшило день?",
		"5. Что завтра обязательно?",
	}, "\n")
}

func formatDailyDirection(direction domain.DailyDirection) string {
	lines := []string{"Сегодня:"}
	for i, anchor := range direction.Anchors {
		if i >= 5 {
			break
		}
		line := fmt.Sprintf("%d. %s", i+1, anchor.Title)
		if anchor.Window != "" {
			line += " — " + anchor.Window
		}
		lines = append(lines, line)
	}
	if len(direction.Priorities) > 0 {
		lines = append(lines, "", "Приоритеты:")
		for i, priority := range direction.Priorities {
			if i >= 3 {
				break
			}
			line := fmt.Sprintf("%d. %s", i+1, priority.Title)
			if priority.Why != "" {
				line += " — " + priority.Why
			}
			lines = append(lines, line)
		}
	}
	if strings.TrimSpace(direction.Text) != "" {
		lines = append(lines, "", direction.Text)
	}
	return strings.Join(lines, "\n")
}

func formatAdaptiveReplanProposal(proposal domain.ReplanProposal) string {
	lines := []string{"Новый план:"}
	if proposal.Reason != "" {
		lines = append(lines, "Причина: "+proposal.Reason)
	}
	if proposal.RiskDetected != "" {
		lines = append(lines, "Риск: "+proposal.RiskDetected)
	}
	if proposal.AuthorityMessage != "" {
		lines = append(lines, "", proposal.AuthorityMessage)
	}

	groups := map[string][]domain.PlanBlock{}
	order := []string{"fixed", "anchor", "flexible", "recovery", "optional"}
	for _, block := range proposal.ProposedPlan.Blocks {
		blockType := block.Type
		if blockType == "" {
			blockType = "flexible"
		}
		groups[blockType] = append(groups[blockType], block)
	}
	for _, blockType := range order {
		blocks := groups[blockType]
		if len(blocks) == 0 {
			continue
		}
		lines = append(lines, "", blockType+":")
		for _, block := range blocks {
			calendarMarker := ""
			if block.CalendarWrite {
				calendarMarker = " [calendar action]"
			}
			lines = append(lines, fmt.Sprintf("- %s, %d мин: %s%s", block.Start, block.DurationMinutes, block.Title, calendarMarker))
		}
	}
	if len(proposal.CalendarActions) > 0 {
		lines = append(lines, "", fmt.Sprintf("Calendar actions к подтверждению: %d", len(proposal.CalendarActions)))
	} else {
		lines = append(lines, "", "Calendar actions: нет.")
	}
	return strings.Join(lines, "\n")
}

func formatDailyReviewSaved(review domain.DailyReview) string {
	lines := []string{"Review сохранил."}
	if review.Summary != "" {
		lines = append(lines, "", "Summary: "+review.Summary)
	}
	if len(review.Patterns) > 0 {
		lines = append(lines, "", "Patterns:")
		for _, pattern := range review.Patterns {
			if pattern.Code == "" {
				continue
			}
			lines = append(lines, "- "+pattern.Code)
		}
	}
	if len(review.TomorrowFocus) > 0 {
		lines = append(lines, "", "Завтра обязательно:")
		for _, focus := range review.TomorrowFocus {
			lines = append(lines, "- "+focus)
		}
	}
	return strings.Join(lines, "\n")
}

func formatReplanProposal(proposal ReplanProposal) string {
	lines := []string{"Новый план:", proposal.Summary}
	for _, item := range proposal.Events {
		marker := ""
		if item.IsFixed {
			marker = " [fixed]"
		}
		lines = append(lines, fmt.Sprintf("- %s - %s: %s%s", item.Start, item.End, item.Title, marker))
	}
	if len(proposal.Notes) > 0 {
		lines = append(lines, "Заметки:")
		for _, note := range proposal.Notes {
			lines = append(lines, "- "+note)
		}
	}
	return strings.Join(lines, "\n")
}

func commandPayload(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	if !strings.HasPrefix(fields[0], "/") {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(strings.TrimPrefix(text, fields[0]))
}

func firstPlanStep(plan domain.ProposedPlan) string {
	for _, block := range plan.Blocks {
		if strings.TrimSpace(block.Title) != "" {
			return block.Title
		}
	}
	return "встать и сделать первый внешний якорь"
}

func startOfWeek(value time.Time) time.Time {
	weekday := int(value.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := value.AddDate(0, 0, -(weekday - 1))
	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
}

func userID(message *telegram.Message) int64 {
	if message.From == nil {
		return 0
	}
	return message.From.ID
}

func username(message *telegram.Message) string {
	if message.From == nil {
		return ""
	}
	return message.From.UserName
}

func chatID(message *telegram.Message) int64 {
	if message == nil || message.Chat == nil {
		return 0
	}
	return message.Chat.ID
}

func callbackChatID(callback *telegram.CallbackQuery) int64 {
	if callback == nil || callback.Message == nil || callback.Message.Chat == nil {
		return 0
	}
	return callback.Message.Chat.ID
}

func callbackUserID(callback *telegram.CallbackQuery) int64 {
	if callback == nil || callback.From == nil {
		return 0
	}
	return callback.From.ID
}
