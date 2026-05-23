package config

import (
	"errors"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL                string
	GoogleCalendarID           string
	GoogleCredentialsFile      string
	GoogleCredentialsJSON      string
	GoogleOAuthRedirectURL     string
	CalendarTokenEncryptionKey string
	HTTPAddr                   string
	OpenAIAPIKey               string
	TelegramBotToken           string
	Timezone                   string
	AutonomyScheduler          bool
	AutonomyDefaultOn          bool
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		DatabaseURL:                os.Getenv("DATABASE_URL"),
		GoogleCalendarID:           getenv("GOOGLE_CALENDAR_ID", "primary"),
		GoogleCredentialsFile:      os.Getenv("GOOGLE_CREDENTIALS_FILE"),
		GoogleCredentialsJSON:      os.Getenv("GOOGLE_CREDENTIALS_JSON"),
		GoogleOAuthRedirectURL:     getenv("GOOGLE_OAUTH_REDIRECT_URL", os.Getenv("GOOGLE_REDIRECT_URL")),
		CalendarTokenEncryptionKey: os.Getenv("CALENDAR_TOKEN_ENCRYPTION_KEY"),
		HTTPAddr:                   getenv("HTTP_ADDR", ":8080"),
		OpenAIAPIKey:               os.Getenv("OPENAI_API_KEY"),
		TelegramBotToken:           os.Getenv("TELEGRAM_BOT_TOKEN"),
		Timezone:                   getenv("APP_TIMEZONE", "Asia/Ho_Chi_Minh"),
		AutonomyScheduler:          getenvBool("AUTONOMY_SCHEDULER_ENABLED", true),
		AutonomyDefaultOn:          getenvBool("AUTONOMY_DEFAULT_ENABLED", false),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.TelegramBotToken == "" {
		return Config{}, errors.New("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.OpenAIAPIKey == "" {
		return Config{}, errors.New("OPENAI_API_KEY is required")
	}

	return cfg, nil
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
