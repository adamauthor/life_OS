package calendar

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type TelegramNotifier interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

type OAuthHTTPServer struct {
	addr     string
	service  *OAuthService
	notifier TelegramNotifier
	logger   *slog.Logger
}

func NewOAuthHTTPServer(addr string, service *OAuthService, notifier TelegramNotifier, logger *slog.Logger) *OAuthHTTPServer {
	if addr == "" {
		addr = ":8080"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OAuthHTTPServer{
		addr:     addr,
		service:  service,
		notifier: notifier,
		logger:   logger,
	}
}

func (s *OAuthHTTPServer) Run(ctx context.Context) error {
	if s.service == nil {
		return fmt.Errorf("oauth service is not configured")
	}
	mux := http.NewServeMux()
	mux.HandleFunc(OAuthCallbackPath, s.handleCallback)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	s.logger.Info("google oauth callback server started", "addr", s.addr, "path", OAuthCallbackPath)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *OAuthHTTPServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		s.logger.Error("google oauth callback rejected", "error", oauthErr)
		http.Error(w, "Calendar connection was not approved. Return to Telegram and run /connect_calendar again.", http.StatusBadRequest)
		return
	}
	result, err := s.service.HandleCallback(r.Context(), r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if err != nil {
		s.logger.Error("google oauth callback failed", "error", err)
		http.Error(w, "Calendar connection failed. Return to Telegram and run /connect_calendar again.", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("Google Calendar connected. You can return to Telegram."))
	if s.notifier != nil && result.ChatID != 0 {
		if err := s.notifier.SendMessage(context.Background(), result.ChatID, "Google Calendar подключен. Теперь /today, /schedule и /replan используют твой календарь."); err != nil {
			s.logger.Error("failed to notify telegram after google oauth", "error", err, "user_id", result.UserID.String())
		}
	}
}
