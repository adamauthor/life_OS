package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL           string
	GoogleCalendarID      string
	GoogleCredentialsFile string
	GoogleCredentialsJSON string
	GoogleTokenFile       string
	GoogleTokenJSON       string
	OpenAIAPIKey          string
	TelegramBotToken      string
	Timezone              string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		GoogleCalendarID:      getenv("GOOGLE_CALENDAR_ID", "primary"),
		GoogleCredentialsFile: os.Getenv("GOOGLE_CREDENTIALS_FILE"),
		GoogleCredentialsJSON: os.Getenv("GOOGLE_CREDENTIALS_JSON"),
		GoogleTokenFile:       os.Getenv("GOOGLE_TOKEN_FILE"),
		GoogleTokenJSON:       os.Getenv("GOOGLE_TOKEN_JSON"),
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		Timezone:              getenv("APP_TIMEZONE", "Asia/Ho_Chi_Minh"),
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
