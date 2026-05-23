package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"life_os/internal/domain"
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
	client   TelegramClient
	logger   *slog.Logger
	memories *MemoryService
	calendar *CalendarService
	reviews  *ReviewService
	ai       AIClient
	timezone *time.Location
}

func NewBot(client TelegramClient, memories *MemoryService, calendar *CalendarService, reviews *ReviewService, ai AIClient, timezone *time.Location, logger *slog.Logger) *Bot {
	if timezone == nil {
		timezone = time.UTC
	}
	return &Bot{
		client:   client,
		logger:   logger,
		memories: memories,
		calendar: calendar,
		reviews:  reviews,
		ai:       ai,
		timezone: timezone,
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
	case "/schedule", "/today":
		return b.today(ctx, msg)
	case "/replan":
		b.handleReplan(ctx, msg)
		return "Принял запрос на перепланирование."
	case "/review":
		return "Короткое ревью дня:\n1. Что сделал?\n2. Что слил?\n3. Что помогло?\n4. Что завтра обязательно?"
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
	action, err := b.calendar.ProposeEvent(ctx, parsed)
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
	answer, err := b.memories.AnswerQuestion(ctx, question)
	if err != nil {
		b.logger.Error("failed to answer memory question", "error", err)
		_ = b.client.SendMessage(ctx, chatID(msg), "Не смог найти ответ в памяти.")
		return
	}
	_ = b.client.SendMessage(ctx, chatID(msg), answer)
}

func (b *Bot) handleReplan(ctx context.Context, msg *telegram.Message) {
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
	action, err := b.calendar.ProposeReplan(ctx, proposal)
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
		eventID, err := b.calendar.ConfirmAction(ctx, id)
		if err != nil {
			b.logger.Error("failed to confirm calendar action", "error", err, "action_id", id)
			_ = b.client.AnswerCallback(ctx, callback.ID, "Ошибка")
			_ = b.client.SendMessage(ctx, chatID, "Не применил событие: "+err.Error())
			return
		}
		_ = b.client.AnswerCallback(ctx, callback.ID, "Готово")
		_ = b.client.SendMessage(ctx, chatID, "Применил в календаре: "+eventID)
	case "cancel":
		if err := b.calendar.CancelAction(ctx, id); err != nil {
			b.logger.Error("failed to cancel calendar action", "error", err, "action_id", id)
		}
		_ = b.client.AnswerCallback(ctx, callback.ID, "Отменено")
		_ = b.client.SendMessage(ctx, chatID, "Ок, не добавляю.")
	case "edit":
		_ = b.client.AnswerCallback(ctx, callback.ID, "Изменить")
		_ = b.client.SendMessage(ctx, chatID, "Напиши исправленное событие одним сообщением: что, когда, длительность.")
	}
}

func (b *Bot) today(ctx context.Context, msg *telegram.Message) string {
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

func (b *Bot) captureTextWithParsed(ctx context.Context, msg *telegram.Message, parsed domain.ParsedIntent) error {
	return b.captureTextWithParsedSource(ctx, msg, parsed, "telegram")
}

func (b *Bot) handleNaturalTextSource(ctx context.Context, msg *telegram.Message, source string) {
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
		"/schedule - события дня",
		"/replan - перестроить день",
		"/today - показать день",
		"/review - daily review",
		"/search <вопрос> - поиск по памяти",
		"/settings - настройки профиля",
		"",
		"Правило: календарь меняю только после подтверждения кнопкой Да.",
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
		"/today - события дня",
		"/replan - перестроить день",
		"/review - daily review",
		"/search <вопрос> - поиск по памяти",
		"/settings - настройки",
		"",
		"Следующий шаг: отправь мысль, задачу, событие или voice.",
	}, "\n")
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
