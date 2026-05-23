package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const pollTimeoutSeconds = 50

type Update = tgbotapi.Update
type Message = tgbotapi.Message
type Voice = tgbotapi.Voice
type Chat = tgbotapi.Chat
type CallbackQuery = tgbotapi.CallbackQuery

type Client struct {
	bot        *tgbotapi.BotAPI
	httpClient *http.Client
}

func NewClient(token string) (*Client, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot api: %w", err)
	}

	return &Client{
		bot: bot,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}, nil
}

func (c *Client) Updates(ctx context.Context) <-chan Update {
	config := tgbotapi.NewUpdate(0)
	config.Timeout = pollTimeoutSeconds

	source := c.bot.GetUpdatesChan(config)
	updates := make(chan Update)

	go func() {
		defer close(updates)
		defer c.bot.StopReceivingUpdates()

		for {
			select {
			case <-ctx.Done():
				return
			case update, ok := <-source:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case updates <- update:
				}
			}
		}
	}()

	return updates
}

func (c *Client) SendMessage(_ context.Context, chatID int64, text string) error {
	message := tgbotapi.NewMessage(chatID, text)
	if _, err := c.bot.Send(message); err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}

	return nil
}

func (c *Client) SendMessageWithButtons(_ context.Context, chatID int64, text string, buttons []InlineButton) error {
	message := tgbotapi.NewMessage(chatID, text)
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(buttons))
	for _, button := range buttons {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(button.Text, button.Data),
		))
	}
	message.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := c.bot.Send(message); err != nil {
		return fmt.Errorf("send telegram message with buttons: %w", err)
	}
	return nil
}

func (c *Client) AnswerCallback(_ context.Context, callbackID string, text string) error {
	config := tgbotapi.NewCallback(callbackID, text)
	if _, err := c.bot.Request(config); err != nil {
		return fmt.Errorf("answer telegram callback: %w", err)
	}
	return nil
}

func (c *Client) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, string, error) {
	url, err := c.bot.GetFileDirectURL(fileID)
	if err != nil {
		return nil, "", fmt.Errorf("get telegram file url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create telegram file request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download telegram file: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		resp.Body.Close()
		return nil, "", fmt.Errorf("telegram file download returned status %d", resp.StatusCode)
	}
	return resp.Body, filenameFromURL(url), nil
}

type InlineButton struct {
	Text string
	Data string
}

func filenameFromURL(rawURL string) string {
	if strings.Contains(rawURL, ".oga") {
		return "voice.oga"
	}
	if strings.Contains(rawURL, ".ogg") {
		return "voice.ogg"
	}
	if strings.Contains(rawURL, ".webm") {
		return "voice.webm"
	}
	return "voice.ogg"
}
