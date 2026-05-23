package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"life_os/internal/ai"
	"life_os/internal/app"
	"life_os/internal/calendar"
	"life_os/internal/companion"
	"life_os/internal/config"
	"life_os/internal/patterns"
	"life_os/internal/planning"
	reviewsvc "life_os/internal/review"
	"life_os/internal/storage"
	"life_os/internal/telegram"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	postgres, err := storage.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect postgres", "error", err)
		os.Exit(1)
	}
	defer postgres.Close()

	client, err := telegram.NewClient(cfg.TelegramBotToken)
	if err != nil {
		logger.Error("failed to create telegram client", "error", err)
		os.Exit(1)
	}

	openAIClient := ai.NewClient(cfg.OpenAIAPIKey)
	memoryRepository := storage.NewMemoryRepository(postgres.Pool)
	memoryService := app.NewMemoryService(memoryRepository, openAIClient)

	calendarRepository := storage.NewCalendarActionRepository(postgres.Pool)
	var calendarClient app.CalendarClient
	if cfg.GoogleCredentialsFile != "" && cfg.GoogleTokenFile != "" {
		googleCalendar, err := calendar.NewGoogleClient(ctx, cfg.GoogleCredentialsFile, cfg.GoogleTokenFile, cfg.GoogleCalendarID)
		if err != nil {
			logger.Warn("google calendar disabled", "error", err)
		} else {
			calendarClient = googleCalendar
		}
	} else if cfg.GoogleCredentialsJSON != "" && cfg.GoogleTokenJSON != "" {
		googleCalendar, err := calendar.NewGoogleClientFromJSON(ctx, cfg.GoogleCredentialsJSON, cfg.GoogleTokenJSON, cfg.GoogleCalendarID)
		if err != nil {
			logger.Warn("google calendar disabled", "error", err)
		} else {
			calendarClient = googleCalendar
		}
	}
	calendarService := app.NewCalendarService(calendarRepository, calendarClient)

	reviewRepository := storage.NewDailyReviewRepository(postgres.Pool)
	reviewService := app.NewReviewService(reviewRepository, memoryRepository, openAIClient)

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		logger.Warn("failed to load timezone, using UTC", "timezone", cfg.Timezone, "error", err)
		location = time.UTC
	}

	adaptiveRepository := storage.NewAdaptiveRepository(postgres.Pool)
	patternService := patterns.NewService(adaptiveRepository)
	companionService := companion.NewService(patternService)
	adaptiveReviewService := reviewsvc.NewService(reviewRepository, adaptiveRepository, memoryRepository, openAIClient, patternService, calendarService, location)
	planningService := planning.NewService(adaptiveRepository, adaptiveRepository, reviewRepository, patternService, calendarService, openAIClient, location)

	bot := app.NewBot(client, memoryService, calendarService, reviewService, openAIClient, location, logger)
	bot.ConfigureAdaptiveServices(planningService, adaptiveReviewService, patternService, companionService)

	if err := bot.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("bot stopped with error", "error", err)
		os.Exit(1)
	}
}
