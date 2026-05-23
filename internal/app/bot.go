package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"life_os/internal/companion"
	"life_os/internal/domain"
	"life_os/internal/notifications"
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
	calendarOAuth  CalendarConnector
	reviews        *ReviewService
	planning       *planning.Service
	reviewV2       *reviewsvc.Service
	patterns       *patterns.Service
	companion      *companion.Service
	notifications  *notifications.Service
	ai             AIClient
	timezone       *time.Location
	pendingReviews map[int64]time.Time
}

type CalendarConnector interface {
	BuildConnectURL(ctx context.Context, userID domain.UUID, chatID int64) (string, error)
	StatusText(ctx context.Context, userID domain.UUID) (string, error)
	Disconnect(ctx context.Context, userID domain.UUID) error
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

func (b *Bot) ConfigureNotificationService(notificationService *notifications.Service) {
	b.notifications = notificationService
}

func (b *Bot) ConfigureCalendarConnector(connector CalendarConnector) {
	if isNilCalendarConnector(connector) {
		b.calendarOAuth = nil
		return
	}
	b.calendarOAuth = connector
}

func (b *Bot) hasCalendarConnector() bool {
	return !isNilCalendarConnector(b.calendarOAuth)
}

func isNilCalendarConnector(connector CalendarConnector) bool {
	if connector == nil {
		return true
	}
	value := reflect.ValueOf(connector)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
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
	b.registerTelegramUser(ctx, msg)
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

	if isCommandText(msg.Text) {
		response := b.routeText(ctx, msg)
		if response != "" {
			if err := b.client.SendMessage(ctx, chatID(msg), response); err != nil {
				b.logger.Error("failed to send telegram message", "error", err, "chat_id", chatID(msg))
			}
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
		b.sendCalendarConnectPrompt(ctx, msg, false)
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
		return ""
	case "/review":
		b.pendingReviews[userID(msg)] = b.localNow(ctx, msg)
		if b.reviewV2 != nil {
			_ = b.reviewV2.StartDailyReview(ctx, domain.UserIDFromTelegram(userID(msg)))
		}
		return dailyReviewQuestions()
	case "/weekly":
		return b.weekly(ctx, msg)
	case "/patterns":
		return b.listPatterns(ctx, msg)
	case "/autonomy":
		return b.handleAutonomyCommand(ctx, msg)
	case "/connect_calendar":
		return b.handleConnectCalendar(ctx, msg)
	case "/calendar_status":
		return b.handleCalendarStatus(ctx, msg)
	case "/disconnect_calendar":
		return b.handleDisconnectCalendar(ctx, msg)
	case "/search":
		query := strings.TrimSpace(strings.TrimPrefix(text, command[0]))
		if query == "" {
			return "Напиши так: /search что я говорил про AI Life OS"
		}
		b.handleMemoryQuestion(ctx, msg, query)
		return "Ищу в памяти."
	case "/settings":
		return b.handleSettingsCommand(ctx, msg)
	default:
		return "Неизвестная команда. Напиши /help."
	}
}

func isCommandText(text string) bool {
	fields := strings.Fields(text)
	return len(fields) > 0 && strings.HasPrefix(fields[0], "/")
}

func (b *Bot) handleNaturalText(ctx context.Context, msg *telegram.Message) {
	b.handleNaturalTextSource(ctx, msg, "telegram")
}

func (b *Bot) registerTelegramUser(ctx context.Context, msg *telegram.Message) {
	if b.notifications == nil || msg == nil || msg.From == nil {
		return
	}
	if _, err := b.notifications.RegisterTelegramUser(ctx, domain.RegisterTelegramUserInput{
		TelegramUserID: msg.From.ID,
		ChatID:         chatID(msg),
		Username:       msg.From.UserName,
		FirstName:      msg.From.FirstName,
		LastName:       msg.From.LastName,
		Timezone:       b.timezone.String(),
	}); err != nil {
		b.logger.Error("failed to register telegram user", "error", err, "telegram_user_id", msg.From.ID)
	}
}

func (b *Bot) localLocation(ctx context.Context, msg *telegram.Message) *time.Location {
	if b.notifications == nil || msg == nil || msg.From == nil {
		return b.timezone
	}
	return b.notifications.UserLocation(ctx, domain.UserIDFromTelegram(msg.From.ID))
}

func (b *Bot) localNow(ctx context.Context, msg *telegram.Message) time.Time {
	return time.Now().In(b.localLocation(ctx, msg))
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
	if clarification := calendarEventClarification(parsed); clarification != "" {
		_ = b.client.SendMessage(ctx, chatID(msg), clarification)
		return
	}
	action, err := b.calendar.ProposeEvent(ctx, domain.UserIDFromTelegram(userID(msg)), parsed)
	if err != nil {
		b.logger.Error("failed to propose calendar event", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), calendarUserErrorText(err, "Не смог подготовить событие. Укажи дату и время явно."))
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

func (b *Bot) handleConnectCalendar(ctx context.Context, msg *telegram.Message) string {
	if !b.hasCalendarConnector() {
		return "Подключение Google Calendar не настроено. Нужны GOOGLE_CREDENTIALS_JSON или GOOGLE_CREDENTIALS_FILE и GOOGLE_OAUTH_REDIRECT_URL."
	}
	if err := b.sendCalendarConnectPrompt(ctx, msg, true); err != nil {
		b.logger.Error("failed to send calendar connect prompt", "error", err)
		return "Не создал ссылку Google Calendar."
	}
	return ""
}

func (b *Bot) sendCalendarConnectPrompt(ctx context.Context, msg *telegram.Message, explicit bool) error {
	if !b.hasCalendarConnector() || msg == nil {
		return nil
	}
	url, err := b.calendarOAuth.BuildConnectURL(ctx, domain.UserIDFromTelegram(userID(msg)), chatID(msg))
	if err != nil {
		return err
	}
	text := "Подключи Google Calendar, чтобы /today, /schedule и /replan учитывали твои события."
	if explicit {
		text = "Подключение Google Calendar.\nОткрой ссылку, выбери свой Google аккаунт и разреши доступ к календарю."
	}
	return b.client.SendMessageWithButtons(ctx, chatID(msg), text, []telegram.InlineButton{
		{Text: "Подключить Google Calendar", URL: url},
	})
}

func (b *Bot) handleCalendarStatus(ctx context.Context, msg *telegram.Message) string {
	if !b.hasCalendarConnector() {
		return "Per-user Google Calendar OAuth не настроен."
	}
	text, err := b.calendarOAuth.StatusText(ctx, domain.UserIDFromTelegram(userID(msg)))
	if err != nil {
		b.logger.Error("failed to read calendar status", "error", err)
		return "Не прочитал статус календаря."
	}
	return text
}

func (b *Bot) handleDisconnectCalendar(ctx context.Context, msg *telegram.Message) string {
	if !b.hasCalendarConnector() {
		return "Per-user Google Calendar OAuth не настроен."
	}
	if err := b.calendarOAuth.Disconnect(ctx, domain.UserIDFromTelegram(userID(msg))); err != nil {
		b.logger.Error("failed to disconnect calendar", "error", err)
		return "Не отключил Google Calendar."
	}
	return "Google Calendar отключен для твоего аккаунта."
}

func (b *Bot) handleAutonomyCommand(ctx context.Context, msg *telegram.Message) string {
	if b.notifications == nil {
		return "Autonomy scheduler не настроен."
	}
	payload := strings.ToLower(strings.TrimSpace(commandPayload(msg.Text)))
	userUUID := domain.UserIDFromTelegram(userID(msg))
	fields := strings.Fields(payload)
	if len(fields) > 0 {
		switch fields[0] {
		case "quiet":
			if len(fields) != 3 {
				return "Формат: /autonomy quiet 23:30 08:00"
			}
			if err := b.notifications.SetQuietHours(ctx, userUUID, fields[1], fields[2]); err != nil {
				b.logger.Error("failed to update autonomy quiet hours", "error", err)
				return "Не обновил quiet hours. Формат времени: HH:MM."
			}
			return "Quiet hours обновлены: " + fields[1] + "-" + fields[2]
		case "limit":
			if len(fields) != 2 {
				return "Формат: /autonomy limit 5"
			}
			limit, err := strconv.Atoi(fields[1])
			if err != nil {
				return "Лимит должен быть числом: /autonomy limit 5"
			}
			if err := b.notifications.SetDailyLimit(ctx, userUUID, limit); err != nil {
				b.logger.Error("failed to update autonomy daily limit", "error", err)
				return "Не обновил лимит. Допустимо от 1 до 12 сообщений в день."
			}
			return fmt.Sprintf("Daily limit обновлен: %d", limit)
		case "time":
			if len(fields) != 3 {
				return "Формат: /autonomy time daily_review 22:30"
			}
			kind, ok := notificationKindFromInput(fields[1])
			if !ok {
				return "Неизвестный тип. Доступно: daily_direction, midday_checkin, pattern_nudge, daily_review, shutdown, weekly_review."
			}
			if err := b.notifications.SetRuleTime(ctx, userUUID, kind, fields[2]); err != nil {
				b.logger.Error("failed to update notification rule time", "error", err, "kind", kind)
				return "Не обновил время. Формат времени: HH:MM."
			}
			return fmt.Sprintf("Время %s обновлено: %s", kind, fields[2])
		}
	}
	switch payload {
	case "on", "enable", "вкл":
		if err := b.notifications.SetEnabled(ctx, userUUID, true); err != nil {
			b.logger.Error("failed to enable autonomy", "error", err)
			return "Не включил autonomy."
		}
		status, err := b.notifications.StatusText(ctx, userUUID)
		if err != nil {
			return "Autonomy включена."
		}
		return "Autonomy включена.\n\n" + status
	case "off", "disable", "выкл":
		if err := b.notifications.SetEnabled(ctx, userUUID, false); err != nil {
			b.logger.Error("failed to disable autonomy", "error", err)
			return "Не выключил autonomy."
		}
		return "Autonomy выключена. Бот не будет писать сам, пока не включишь /autonomy on."
	case "", "status":
		status, err := b.notifications.StatusText(ctx, userUUID)
		if err != nil {
			b.logger.Error("failed to read autonomy status", "error", err)
			return "Не прочитал autonomy settings."
		}
		return status
	default:
		return strings.Join([]string{
			"Команды autonomy:",
			"/autonomy on - включить автономные напоминания",
			"/autonomy off - выключить",
			"/autonomy status - статус",
			"/autonomy quiet 23:30 08:00 - не писать в тихие часы",
			"/autonomy limit 5 - максимум сообщений в день",
			"/autonomy time daily_review 22:30 - время конкретного напоминания",
			"",
			"Календарь сам не меняю. Только напоминания, check-ins и предложения.",
		}, "\n")
	}
}

func (b *Bot) handleSettingsCommand(ctx context.Context, msg *telegram.Message) string {
	payload := strings.TrimSpace(commandPayload(msg.Text))
	userUUID := domain.UserIDFromTelegram(userID(msg))
	if payload == "" || strings.EqualFold(payload, "status") {
		loc := b.localLocation(ctx, msg)
		now := time.Now().In(loc)
		return strings.Join([]string{
			"Настройки профиля:",
			"Timezone: " + loc.String(),
			"Локальное время: " + now.Format("2006-01-02 15:04"),
			"",
			"Команды:",
			"/settings timezone Asia/Ho_Chi_Minh",
			"/settings timezone Europe/Moscow",
			"",
			"Telegram не отдает timezone смартфона автоматически. Укажи IANA timezone один раз, дальше today/replan/calendar/review будут считать время по ней.",
		}, "\n")
	}

	fields := strings.Fields(payload)
	if len(fields) == 2 && strings.EqualFold(fields[0], "timezone") {
		loc, err := time.LoadLocation(fields[1])
		if err != nil {
			return "Не понял timezone. Нужен IANA формат, например Asia/Ho_Chi_Minh или Europe/Moscow."
		}
		if b.notifications == nil {
			return "Хранилище пользователя не настроено. Пока timezone берется из APP_TIMEZONE."
		}
		if err := b.notifications.SetUserTimezone(ctx, userUUID, loc.String()); err != nil {
			b.logger.Error("failed to update user timezone", "error", err, "user_id", userUUID)
			return "Не сохранил timezone."
		}
		return "Timezone обновлен: " + loc.String() + ". Локальное время: " + time.Now().In(loc).Format("2006-01-02 15:04")
	}

	return "Формат: /settings timezone Asia/Ho_Chi_Minh"
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
		proposal, err := b.planning.BuildReplanProposal(ctx, domain.UserIDFromTelegram(userID(msg)), input, b.localNow(ctx, msg))
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
	dayEvents, err := b.calendar.ListDayForUser(ctx, domain.UserIDFromTelegram(userID(msg)), b.localNow(ctx, msg))
	if err != nil {
		b.logger.Error("failed to list calendar events for replan", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), calendarUserErrorText(err, "Не смог прочитать календарь для перепланирования."))
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
	if _, err := b.reviews.SaveDailyReview(ctx, msg.Text, b.localNow(ctx, msg)); err != nil {
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
	if strings.HasPrefix(callback.Data, "notify_") {
		b.handleNotificationCallback(ctx, callback)
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
			_ = b.client.AnswerCallback(ctx, callback.ID, "Ошибка")
			_ = b.client.SendMessage(ctx, chatID, "Не отменил действие: "+err.Error())
			return
		}
		_ = b.client.AnswerCallback(ctx, callback.ID, "Отменено")
		_ = b.client.SendMessage(ctx, chatID, "Ок, не добавляю.")
	case "edit":
		_ = b.client.AnswerCallback(ctx, callback.ID, "Изменить")
		_ = b.client.SendMessage(ctx, chatID, "Напиши исправленное событие одним сообщением: что, когда, длительность.")
	}
}

func (b *Bot) handleNotificationCallback(ctx context.Context, callback *telegram.CallbackQuery) {
	if b.notifications == nil {
		_ = b.client.AnswerCallback(ctx, callback.ID, "Autonomy не настроена")
		return
	}
	action, notificationID, err := parseNotificationCallback(callback.Data)
	if err != nil {
		_ = b.client.AnswerCallback(ctx, callback.ID, "Некорректное действие")
		return
	}
	userUUID := domain.UserIDFromTelegram(callbackUserID(callback))
	chatID := callbackChatID(callback)
	answerText := ""

	switch action {
	case "done":
		err = b.notifications.MarkDone(ctx, userUUID, notificationID)
		answerText = "Отмечено"
	case "skip":
		err = b.notifications.MarkSkipped(ctx, userUUID, notificationID)
		answerText = "Пропущено"
	case "snooze30":
		err = b.notifications.Snooze(ctx, userUUID, notificationID, 30*time.Minute)
		answerText = "Отложено на 30 минут"
	case "snooze120":
		err = b.notifications.Snooze(ctx, userUUID, notificationID, 2*time.Hour)
		answerText = "Отложено на 2 часа"
	case "review":
		err = b.notifications.MarkDone(ctx, userUUID, notificationID)
		if err == nil {
			b.pendingReviews[callbackUserID(callback)] = time.Now().In(b.timezone)
			_ = b.client.SendMessage(ctx, chatID, dailyReviewQuestions())
		}
		answerText = "Review"
	case "replan":
		proposal, buildErr := b.notifications.BuildReplanFromNotification(ctx, userUUID, notificationID)
		if buildErr != nil {
			err = buildErr
			break
		}
		text := formatAdaptiveReplanProposal(*proposal)
		err = b.client.SendMessageWithButtons(ctx, chatID, text+"\n\nПрименить изменения в календаре?", []telegram.InlineButton{
			{Text: "Применить", Data: fmt.Sprintf("replan_confirm:%s", proposal.ID.String())},
			{Text: "Изменить", Data: fmt.Sprintf("replan_edit:%s", proposal.ID.String())},
			{Text: "Отклонить", Data: fmt.Sprintf("replan_cancel:%s", proposal.ID.String())},
		})
		answerText = "Replan готов"
	default:
		_ = b.client.AnswerCallback(ctx, callback.ID, "Неизвестное действие")
		return
	}
	if err != nil {
		b.logger.Error("failed to handle notification callback", "error", err, "action", action, "notification_id", notificationID.String())
		_ = b.client.AnswerCallback(ctx, callback.ID, "Ошибка")
		_ = b.client.SendMessage(ctx, chatID, "Не применил действие: "+err.Error())
		return
	}
	if answerText != "" {
		_ = b.client.AnswerCallback(ctx, callback.ID, answerText)
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
			_ = b.client.AnswerCallback(ctx, callback.ID, "Ошибка")
			_ = b.client.SendMessage(ctx, chatID, "Не отменил replan: "+err.Error())
			return
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
	events, err := b.calendar.ListDayForUser(ctx, domain.UserIDFromTelegram(userID(msg)), b.localNow(ctx, msg))
	if err != nil {
		b.logger.Error("failed to list today calendar", "error", err)
		return calendarUserErrorText(err, "Не смог прочитать календарь. Проверь GOOGLE_CALENDAR_ID: для основного календаря используй primary.")
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
	direction, err := b.planning.BuildDailyDirection(ctx, domain.UserIDFromTelegram(userID(msg)), b.localNow(ctx, msg))
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
	now := b.localNow(ctx, msg)
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
	loc := b.localLocation(ctx, msg)
	now := time.Now().In(loc)
	parsed, err := b.ai.ParseIntent(ctx, msg.Text, now.Format(time.RFC3339), loc.String())
	if err != nil {
		b.logger.Error("failed to parse intent", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не разобрал намерение. Переформулируй короче.")
		return
	}
	parsed = normalizeParsedIntent(msg.Text, parsed)
	parsed = completeCalendarIntentFromText(msg.Text, parsed, now)
	b.sendRecognitionNotice(ctx, msg, source, parsed)

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
		_ = b.client.SendMessage(ctx, chatID(msg), "Сохранил в память как "+memoryTypeLabel(parsed.Type)+".")
	}
}

func (b *Bot) sendRecognitionNotice(ctx context.Context, msg *telegram.Message, source string, parsed domain.ParsedIntent) {
	if source != "telegram_voice" {
		return
	}
	lines := []string{
		"Распознал voice:",
		msg.Text,
		"",
		"Категория: " + intentLabel(parsed),
		"Дальше: " + intentNextStep(parsed.Intent),
	}
	_ = b.client.SendMessage(ctx, chatID(msg), strings.Join(lines, "\n"))
}

func (b *Bot) captureTextWithParsedSource(ctx context.Context, msg *telegram.Message, parsed domain.ParsedIntent, source string) error {
	loc := b.localLocation(ctx, msg)
	now := time.Now().In(loc)
	_, err := b.memories.CaptureParsedTelegramText(ctx, CaptureTelegramTextInput{
		Text:       msg.Text,
		Source:     source,
		ChatID:     chatID(msg),
		MessageID:  msg.MessageID,
		UserID:     userID(msg),
		Username:   username(msg),
		TelegramAt: msg.Date,
		Timezone:   loc.String(),
		NowRFC3339: now.Format(time.RFC3339),
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
		"После voice я показываю распознанный текст, категорию и следующий шаг.",
		"",
		"Что умею распознавать:",
		"- память: идеи, задачи, заметки, рефлексии",
		"- событие календаря: предложу и спрошу подтверждение",
		"- replan: перестрою день и спрошу подтверждение перед изменениями",
		"- вопрос к памяти: найду по embeddings",
		"- daily/weekly review: сохраню выводы и patterns",
		"",
		"Что не умею:",
		"- не меняю календарь без кнопки подтверждения",
		"- не отправляю внешние сообщения",
		"- не трекаю Apple Health, Screen Time, Obsidian, Web UI",
		"- не гарантирую идеальный разбор двусмысленного текста; если непонятно, скажи явно: идея, задача, событие, вопрос или replan",
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
		"/autonomy - автономные напоминания",
		"/connect_calendar - подключить Google Calendar",
		"/calendar_status - статус календаря",
		"/disconnect_calendar - отключить Google Calendar",
		"/search <вопрос> - поиск по памяти",
		"/settings - timezone и настройки профиля",
		"/settings timezone Asia/Ho_Chi_Minh - сохранить локальное время",
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
		"3. После voice покажу распознанный текст, категорию и следующий шаг.",
		"4. Календарь меняю только после твоего подтверждения.",
		"",
		"Дисклеймер:",
		"- Я не отправляю внешние сообщения.",
		"- Я не меняю календарь без кнопки подтверждения.",
		"- Если запрос двусмысленный, скажи явно: идея, задача, событие, вопрос, review или replan.",
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
		"/autonomy on - включить автономные напоминания",
		"/connect_calendar - подключить личный Google Calendar",
		"/search <вопрос> - поиск по памяти",
		"/settings - timezone и настройки",
		"/settings timezone Asia/Ho_Chi_Minh - сохранить локальное время",
		"",
		"Следующий шаг: отправь мысль, задачу, событие или voice.",
	}, "\n")
}

func intentLabel(parsed domain.ParsedIntent) string {
	switch parsed.Intent {
	case domain.IntentCaptureMemory:
		return "память / " + memoryTypeLabel(parsed.Type)
	case domain.IntentCreateTask:
		return "задача"
	case domain.IntentCreateCalendarEvent:
		return "событие календаря"
	case domain.IntentReplanDay:
		return "перепланирование дня"
	case domain.IntentAskMemory:
		return "вопрос к памяти"
	case domain.IntentDailyReview:
		return "daily review"
	case domain.IntentWeeklyReview:
		return "weekly review"
	case domain.IntentHabitLog:
		return "habit log"
	default:
		return "непонятно"
	}
}

func intentNextStep(intent domain.Intent) string {
	switch intent {
	case domain.IntentCaptureMemory, domain.IntentCreateTask, domain.IntentHabitLog:
		return "сохраню в память, если это действительно заметка/идея/задача."
	case domain.IntentCreateCalendarEvent:
		return "подготовлю событие и попрошу подтверждение."
	case domain.IntentReplanDay:
		return "прочитаю календарь, предложу новый план и попрошу подтверждение."
	case domain.IntentAskMemory:
		return "поищу по памяти и отвечу с учетом найденного."
	case domain.IntentDailyReview:
		return "сохраню review, summary и patterns."
	case domain.IntentWeeklyReview:
		return "соберу weekly review."
	default:
		return "попрошу уточнить формат."
	}
}

func memoryTypeLabel(memoryType domain.MemoryType) string {
	switch memoryType {
	case domain.MemoryTypeIdea:
		return "идея"
	case domain.MemoryTypeTask:
		return "задача"
	case domain.MemoryTypeReflection:
		return "рефлексия"
	case domain.MemoryTypeEvent:
		return "событие"
	case domain.MemoryTypeQuestion:
		return "вопрос"
	default:
		return "заметка"
	}
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

func parseNotificationCallback(data string) (string, uuid.UUID, error) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return "", uuid.Nil, fmt.Errorf("invalid notification callback")
	}
	action := strings.TrimPrefix(parts[0], "notify_")
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return "", uuid.Nil, err
	}
	return action, id, nil
}

func notificationKindFromInput(value string) (domain.NotificationKind, bool) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "daily_direction", "direction", "today", "morning":
		return domain.NotificationKindDailyDirection, true
	case "midday_checkin", "midday", "checkin":
		return domain.NotificationKindMiddayCheckin, true
	case "pattern_nudge", "pattern", "nudge":
		return domain.NotificationKindPatternNudge, true
	case "daily_review", "review":
		return domain.NotificationKindDailyReview, true
	case "shutdown", "sleep":
		return domain.NotificationKindShutdown, true
	case "weekly_review", "weekly":
		return domain.NotificationKindWeeklyReview, true
	default:
		return "", false
	}
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

func calendarUserErrorText(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	message := err.Error()
	if strings.Contains(message, "calendar is not connected for this user") {
		return "Календарь для твоего аккаунта пока не подключен. Подключи: /connect_calendar. Остальные функции работают: память, review, patterns, /today и /replan без записи в календарь."
	}
	if strings.Contains(message, "calendar adapter is not configured") {
		return "Календарь не настроен. Остальные функции работают без календарных действий."
	}
	return fallback
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
