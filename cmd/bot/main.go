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
	assistantpkg "life_os/internal/assistant"
	"life_os/internal/calendar"
	"life_os/internal/companion"
	"life_os/internal/config"
	"life_os/internal/notifications"
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
	assistantRepository := storage.NewAssistantRepository(postgres.Pool)
	userProfileService := assistantpkg.NewUserProfileService(assistantRepository)
	knowledgeService := assistantpkg.NewKnowledgeService(assistantRepository, openAIClient)
	anchorService := assistantpkg.NewAnchorService(assistantRepository)

	calendarRepository := storage.NewCalendarActionRepository(postgres.Pool)
	calendarService := app.NewCalendarService(calendarRepository, nil)
	var calendarOAuth *calendar.OAuthService
	googleCredentialsJSON := cfg.GoogleCredentialsJSON
	if googleCredentialsJSON == "" && cfg.GoogleCredentialsFile != "" {
		credentials, err := os.ReadFile(cfg.GoogleCredentialsFile)
		if err != nil {
			logger.Warn("google calendar oauth disabled: failed to read credentials file", "error", err)
		} else {
			googleCredentialsJSON = string(credentials)
		}
	}
	if googleCredentialsJSON != "" && cfg.GoogleOAuthRedirectURL != "" {
		googleCalendarRepository := storage.NewGoogleCalendarRepository(postgres.Pool)
		service, err := calendar.NewOAuthService(googleCalendarRepository, googleCredentialsJSON, cfg.GoogleOAuthRedirectURL, cfg.GoogleCalendarID, cfg.CalendarTokenEncryptionKey)
		if err != nil {
			logger.Warn("google calendar oauth disabled", "error", err)
		} else {
			calendarOAuth = service
			calendarService.ConfigureProvider(service)
		}
	} else if googleCredentialsJSON != "" {
		logger.Warn("google calendar oauth disabled: GOOGLE_OAUTH_REDIRECT_URL is not set")
	}

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
	notificationRepository := storage.NewNotificationRepository(postgres.Pool)
	notificationService := notifications.NewService(
		notificationRepository,
		client,
		planningService,
		adaptiveReviewService,
		patternService,
		location,
		logger,
		notifications.Config{
			SchedulerEnabled: cfg.AutonomyScheduler,
			DefaultEnabled:   cfg.AutonomyDefaultOn,
		},
	)

	bot := app.NewBot(client, memoryService, calendarService, reviewService, openAIClient, location, logger)
	bot.ConfigureAdaptiveServices(planningService, adaptiveReviewService, patternService, companionService)
	bot.ConfigureNotificationService(notificationService)
	bot.ConfigureCalendarConnector(calendarOAuth)
	bot.ConfigureAssistantService(&assistantpkg.Service{
		AI:          openAIClient,
		Calendar:    calendarService,
		Knowledge:   knowledgeService,
		UserProfile: userProfileService,
		Anchors:     anchorService,
		Planning:    planningService,
		Memory:      memoryService,
	})

	go notificationService.Run(ctx)
	if calendarOAuth != nil {
		oauthServer := calendar.NewOAuthHTTPServer(cfg.HTTPAddr, calendarOAuth, client, logger)
		go func() {
			if err := oauthServer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("google oauth server stopped with error", "error", err)
			}
		}()
	}

	if err := bot.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("bot stopped with error", "error", err)
		os.Exit(1)
	}
}
