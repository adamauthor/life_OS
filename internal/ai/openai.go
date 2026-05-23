package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"

	"life_os/internal/app"
	"life_os/internal/domain"
)

type Client struct {
	openai openai.Client
	model  shared.ChatModel
}

func NewClient(apiKey string) *Client {
	return &Client{
		openai: openai.NewClient(option.WithAPIKey(apiKey)),
		model:  shared.ChatModelGPT4_1Mini,
	}
}

func (c *Client) ParseIntent(ctx context.Context, text string, nowRFC3339 string, timezone string) (domain.ParsedIntent, error) {
	if strings.TrimSpace(nowRFC3339) == "" {
		nowRFC3339 = time.Now().Format(time.RFC3339)
	}
	if strings.TrimSpace(timezone) == "" {
		timezone = "UTC"
	}

	prompt := fmt.Sprintf(`Return strict JSON only.
Current time: %s
Timezone: %s

Classify the user message into one of:
capture_memory, create_task, create_calendar_event, replan_day, ask_memory, daily_review, weekly_review, habit_log, unknown.

Memory type must be one of: idea, task, note, reflection, event, question.
Use capture_memory only when the user is dumping a thought, idea, note, reflection, knowledge, or explicit task to remember.
Do not use capture_memory for questions, calendar requests, or day replanning.
Use ask_memory when the user asks what they previously said, thought, wrote, planned, or remembered.
Use create_calendar_event when the user asks to schedule/add/plan an event at a date or time.
Use replan_day when the user asks to rebuild/reschedule/replan the day, including being late or waking up late.
For calendar writes, requires_confirmation must be true.
For calendar events, include title, datetime in RFC3339 with timezone, duration_minutes.
For memory capture, include summary and tags.

Examples:
"идея: сервис учета калорий как бюджет" => capture_memory, type idea
"что я говорил про AI Life OS" => ask_memory, type question
"завтра в 11 разобрать Kafka consumer groups" => create_calendar_event, type event, requires_confirmation true
"я проспал, сейчас 11:40, перестрой день" => replan_day, requires_confirmation true

User message:
%s`, nowRFC3339, timezone, text)

	var parsed domain.ParsedIntent
	if err := c.chatJSON(ctx, prompt, &parsed); err != nil {
		return domain.ParsedIntent{}, err
	}
	if !parsed.Intent.Valid() {
		parsed.Intent = domain.IntentUnknown
	}
	if parsed.Type != "" && !parsed.Type.Valid() {
		parsed.Type = domain.MemoryTypeNote
	}
	if parsed.RawText == "" {
		parsed.RawText = text
	}
	if parsed.Intent == domain.IntentCreateCalendarEvent {
		parsed.RequiresConfirmation = true
	}
	return parsed, nil
}

func (c *Client) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	result, err := c.openai.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
		Model:          openai.EmbeddingModelTextEmbedding3Small,
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})
	if err != nil {
		return nil, fmt.Errorf("openai embeddings: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("openai embeddings returned no data")
	}

	vector := make([]float32, len(result.Data[0].Embedding))
	for i, value := range result.Data[0].Embedding {
		vector[i] = float32(value)
	}
	return vector, nil
}

func (c *Client) Transcribe(ctx context.Context, filename string, audio io.Reader) (string, error) {
	filename, contentType := audioFileMetadata(filename)
	result, err := c.openai.Audio.Transcriptions.New(ctx, openai.AudioTranscriptionNewParams{
		File:           openai.File(audio, filename, contentType),
		Model:          openai.AudioModelWhisper1,
		Language:       openai.String("ru"),
		Prompt:         openai.String("Russian personal assistant notes, calendar events, daily planning, tasks, ideas."),
		ResponseFormat: openai.AudioResponseFormatJSON,
	})
	if err != nil {
		return "", fmt.Errorf("openai transcription: %w", err)
	}
	return strings.TrimSpace(result.Text), nil
}

func audioFileMetadata(filename string) (string, string) {
	if strings.TrimSpace(filename) == "" {
		return "voice.ogg", "audio/ogg"
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".ogg", ".oga":
		return filename, "audio/ogg"
	case ".webm":
		return filename, "audio/webm"
	case ".mp3":
		return filename, "audio/mpeg"
	case ".m4a", ".mp4":
		return filename, "audio/mp4"
	case ".wav":
		return filename, "audio/wav"
	case ".flac":
		return filename, "audio/flac"
	default:
		return filename + ".ogg", "audio/ogg"
	}
}

func (c *Client) AnswerWithMemories(ctx context.Context, question string, memories []domain.Memory) (string, error) {
	var builder strings.Builder
	for _, memory := range memories {
		builder.WriteString("- ")
		builder.WriteString(memory.Summary)
		builder.WriteString("\nRaw: ")
		builder.WriteString(memory.RawText)
		builder.WriteString("\n")
	}

	prompt := fmt.Sprintf(`Answer in Russian, short and direct. Use only the supplied memory context. If context is insufficient, say that directly.

Question: %s

Memory context:
%s`, question, builder.String())

	return c.chatText(ctx, prompt)
}

func (c *Client) SummarizeDailyReview(ctx context.Context, rawText string) (domain.DailyReview, error) {
	var parsed struct {
		Summary  string   `json:"summary"`
		Mood     string   `json:"mood"`
		Energy   int      `json:"energy"`
		Wins     []string `json:"wins"`
		Failures []string `json:"failures"`
		Patterns []string `json:"patterns"`
	}
	prompt := fmt.Sprintf(`Return strict JSON only. Summarize this daily review.
Fields: summary string, mood string, energy integer 1-10 if known else 0, wins array, failures array, patterns array.

Review:
%s`, rawText)
	if err := c.chatJSON(ctx, prompt, &parsed); err != nil {
		return domain.DailyReview{}, err
	}
	return domain.DailyReview{
		Summary:  parsed.Summary,
		Mood:     parsed.Mood,
		Energy:   parsed.Energy,
		Wins:     parsed.Wins,
		Failures: parsed.Failures,
		Patterns: parsed.Patterns,
	}, nil
}

func (c *Client) ReplanDay(ctx context.Context, message string, calendarEvents []app.CalendarEvent) (app.ReplanProposal, error) {
	events, err := json.Marshal(calendarEvents)
	if err != nil {
		return app.ReplanProposal{}, fmt.Errorf("marshal calendar events: %w", err)
	}
	prompt := fmt.Sprintf(`Return strict JSON only.
You are an authority-driven life companion. Be short, direct, non-toxic, action-focused.
Create a revised day plan in Russian.
Respect fixed events. Fixed events must have is_fixed=true and action="keep".
For movable existing events, include source_event_id and action="update".
For new recommended calendar blocks, leave source_event_id empty and action="create".
Do not include deletes in MVP.

JSON schema:
{
  "summary": "short Russian plan summary",
  "events": [
    {
      "source_event_id": "calendar event id or empty",
      "title": "event title",
      "start": "RFC3339 datetime",
      "end": "RFC3339 datetime",
      "is_fixed": false,
      "action": "keep|update|create"
    }
  ],
  "notes": ["short direct notes"]
}

User request:
%s

Calendar events JSON:
%s`, message, string(events))
	var proposal app.ReplanProposal
	if err := c.chatJSON(ctx, prompt, &proposal); err != nil {
		return app.ReplanProposal{}, err
	}
	return proposal, nil
}

func (c *Client) chatJSON(ctx context.Context, prompt string, out any) error {
	jsonObject := shared.NewResponseFormatJSONObjectParam()
	chat, err := c.openai.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You return valid JSON only. No markdown. No commentary."),
			openai.UserMessage(prompt),
		},
		Model: c.model,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &jsonObject,
		},
	})
	if err != nil {
		return fmt.Errorf("openai chat json: %w", err)
	}
	if len(chat.Choices) == 0 {
		return fmt.Errorf("openai chat returned no choices")
	}
	if err := json.Unmarshal([]byte(chat.Choices[0].Message.Content), out); err != nil {
		return fmt.Errorf("decode openai json: %w", err)
	}
	return nil
}

func (c *Client) chatText(ctx context.Context, prompt string) (string, error) {
	chat, err := c.openai.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("Ты authority-driven companion. Коротко, прямо, без токсичности. Всегда дай следующий шаг."),
			openai.UserMessage(prompt),
		},
		Model: c.model,
	})
	if err != nil {
		return "", fmt.Errorf("openai chat text: %w", err)
	}
	if len(chat.Choices) == 0 {
		return "", fmt.Errorf("openai chat returned no choices")
	}
	return strings.TrimSpace(chat.Choices[0].Message.Content), nil
}
