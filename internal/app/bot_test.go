package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"life_os/internal/domain"
	"life_os/internal/telegram"
)

type noopTelegramClient struct{}

func (noopTelegramClient) Updates(_ context.Context) <-chan telegram.Update {
	return make(chan telegram.Update)
}

func (noopTelegramClient) SendMessage(_ context.Context, _ int64, _ string) error {
	return nil
}

func (noopTelegramClient) SendMessageWithButtons(_ context.Context, _ int64, _ string, _ []telegram.InlineButton) error {
	return nil
}

func (noopTelegramClient) AnswerCallback(_ context.Context, _ string, _ string) error {
	return nil
}

func (noopTelegramClient) DownloadFile(_ context.Context, _ string) (io.ReadCloser, string, error) {
	return nil, "", nil
}

type recordingTelegramClient struct {
	downloadedFileID string
	buttonMessages   []buttonMessage
	messages         []string
}

type buttonMessage struct {
	text    string
	buttons []telegram.InlineButton
}

func (c *recordingTelegramClient) Updates(_ context.Context) <-chan telegram.Update {
	return make(chan telegram.Update)
}

func (c *recordingTelegramClient) SendMessage(_ context.Context, _ int64, text string) error {
	c.messages = append(c.messages, text)
	return nil
}

func (c *recordingTelegramClient) SendMessageWithButtons(_ context.Context, _ int64, text string, buttons []telegram.InlineButton) error {
	c.buttonMessages = append(c.buttonMessages, buttonMessage{text: text, buttons: buttons})
	return nil
}

func (c *recordingTelegramClient) AnswerCallback(_ context.Context, _ string, _ string) error {
	return nil
}

func (c *recordingTelegramClient) DownloadFile(_ context.Context, fileID string) (io.ReadCloser, string, error) {
	c.downloadedFileID = fileID
	return io.NopCloser(bytes.NewBufferString("voice")), "voice.ogg", nil
}

type voiceFirstAI struct{}

func (voiceFirstAI) ParseIntent(_ context.Context, text string, _ string, _ string) (domain.ParsedIntent, error) {
	if strings.Contains(text, "перестрой день") {
		return domain.ParsedIntent{Intent: domain.IntentReplanDay, Confidence: 0.95}, nil
	}
	return domain.ParsedIntent{Intent: domain.IntentCaptureMemory, Type: domain.MemoryTypeNote, Summary: text}, nil
}

func (voiceFirstAI) CreateEmbedding(_ context.Context, _ string) ([]float32, error) {
	return make([]float32, 1536), nil
}

func (voiceFirstAI) Transcribe(_ context.Context, _ string, _ io.Reader) (string, error) {
	return "я проспал, время сейчас 11:40 перестрой день", nil
}

func (voiceFirstAI) AnswerWithMemories(_ context.Context, _ string, _ []domain.Memory) (string, error) {
	return "", nil
}

func (voiceFirstAI) SummarizeDailyReview(_ context.Context, _ string) (domain.DailyReview, error) {
	return domain.DailyReview{}, nil
}

func (voiceFirstAI) ReplanDay(_ context.Context, _ string, _ []CalendarEvent) (ReplanProposal, error) {
	return ReplanProposal{
		Summary: "Сдвинь день без самообмана.",
		Events: []ReplanProposalItem{
			{SourceEventID: "movable", Title: "Deep work", Start: "2026-05-23T12:00:00+07:00", End: "2026-05-23T14:00:00+07:00", Action: "update"},
		},
		Notes: []string{"Следующий шаг: начать с одного важного блока."},
	}, nil
}

type fakeCalendarConnector struct{}

func (fakeCalendarConnector) BuildConnectURL(_ context.Context, _ domain.UUID, _ int64) (string, error) {
	return "https://example.com/oauth", nil
}

func (fakeCalendarConnector) StatusText(_ context.Context, _ domain.UUID) (string, error) {
	return "Google Calendar подключен.", nil
}

func (fakeCalendarConnector) Disconnect(_ context.Context, _ domain.UUID) error {
	return nil
}

func TestRouteTextCommands(t *testing.T) {
	bot := NewBot(noopTelegramClient{}, nil, nil, nil, nil, time.UTC, slog.Default())

	tests := map[string]string{
		"/start":    "Adaptive Life Companion включен.",
		"/help":     "Команды:",
		"/capture":  "Пришли мысль",
		"/schedule": "Календарь не настроен.",
		"/replan":   "Принял запрос",
		"/today":    "Календарь не настроен.",
		"/review":   "Короткое ревью дня:",
		"/search":   "Напиши так:",
		"/settings": "Настройки профиля",
		"/unknown":  "Неизвестная команда.",
	}

	for input, wantContains := range tests {
		t.Run(input, func(t *testing.T) {
			got := bot.routeText(context.Background(), &telegram.Message{Text: input})
			if !strings.Contains(got, wantContains) {
				t.Fatalf("routeText(%q) = %q, want substring %q", input, got, wantContains)
			}
		})
	}
}

func TestVoiceMessageRunsNaturalReplanWithoutCommand(t *testing.T) {
	telegramClient := &recordingTelegramClient{}
	calendarRepository := newFakeCalendarRepository()
	calendarClient := &fakeCalendarClient{}
	calendarService := NewCalendarService(calendarRepository, calendarClient)
	bot := NewBot(telegramClient, nil, calendarService, nil, voiceFirstAI{}, time.UTC, slog.Default())

	bot.handleUpdate(context.Background(), telegram.Update{
		UpdateID: 1,
		Message: &telegram.Message{
			MessageID: 10,
			Chat:      &telegram.Chat{ID: 123},
			Voice:     &telegram.Voice{FileID: "voice-file-id"},
		},
	})

	if telegramClient.downloadedFileID != "voice-file-id" {
		t.Fatalf("downloadedFileID = %q", telegramClient.downloadedFileID)
	}
	if len(telegramClient.buttonMessages) != 1 {
		t.Fatalf("buttonMessages len = %d, want 1", len(telegramClient.buttonMessages))
	}
	got := telegramClient.buttonMessages[0]
	if !strings.Contains(got.text, "Новый план:") {
		t.Fatalf("proposal text = %q, want replan proposal", got.text)
	}
	if len(got.buttons) == 0 || got.buttons[0].Text != "Да" {
		t.Fatalf("buttons = %#v, want confirm button", got.buttons)
	}
}

func TestRouteTextNonCommand(t *testing.T) {
	bot := NewBot(noopTelegramClient{}, nil, nil, nil, nil, time.UTC, slog.Default())

	if got := bot.routeText(context.Background(), &telegram.Message{Text: "идея: AI Life OS"}); got != "" {
		t.Fatalf("routeText returned %q, want empty response for non-command text", got)
	}
}

func TestConnectCalendarCommandSendsURLButton(t *testing.T) {
	telegramClient := &recordingTelegramClient{}
	bot := NewBot(telegramClient, nil, nil, nil, nil, time.UTC, slog.Default())
	bot.ConfigureCalendarConnector(fakeCalendarConnector{})

	response := bot.routeText(context.Background(), &telegram.Message{
		Text: "/connect_calendar",
		Chat: &telegram.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456},
	})
	if response != "" {
		t.Fatalf("response = %q, want empty because command sends button directly", response)
	}
	if len(telegramClient.buttonMessages) != 1 {
		t.Fatalf("buttonMessages len = %d, want 1", len(telegramClient.buttonMessages))
	}
	buttons := telegramClient.buttonMessages[0].buttons
	if len(buttons) != 1 || buttons[0].URL != "https://example.com/oauth" {
		t.Fatalf("buttons = %#v, want oauth URL button", buttons)
	}
}
