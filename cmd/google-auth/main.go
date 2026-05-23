package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcalendar "google.golang.org/api/calendar/v3"
)

const callbackPath = "/oauth2callback"

func main() {
	_ = godotenv.Load()

	credentialsFile := getenv("GOOGLE_CREDENTIALS_FILE", "client_secret_google_calendar.json")
	tokenFile := getenv("GOOGLE_TOKEN_FILE", "google_token_calendar.json")
	redirectURL := getenv("GOOGLE_REDIRECT_URL", "http://localhost:8085/oauth2callback")

	credentials, err := os.ReadFile(credentialsFile)
	if err != nil {
		log.Fatalf("read credentials file: %v", err)
	}

	config, err := google.ConfigFromJSON(credentials, gcalendar.CalendarScope)
	if err != nil {
		log.Fatalf("parse credentials file: %v", err)
	}
	config.RedirectURL = redirectURL

	token, err := getToken(context.Background(), config)
	if err != nil {
		log.Fatalf("get oauth token: %v", err)
	}
	if err := saveToken(tokenFile, token); err != nil {
		log.Fatalf("save token: %v", err)
	}

	fmt.Printf("Google Calendar token saved to %s\n", tokenFile)
}

func getToken(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	state := fmt.Sprintf("life-os-%d", time.Now().UnixNano())
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			errCh <- errors.New("invalid oauth state")
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- errors.New("missing oauth code")
			return
		}
		fmt.Fprintln(w, "OAuth complete. You can return to the terminal.")
		codeCh <- code
	})

	listener, err := net.Listen("tcp", "localhost:8085")
	if err != nil {
		return nil, fmt.Errorf("listen on localhost:8085: %w", err)
	}
	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	defer server.Shutdown(ctx)

	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("Open this URL in your browser and approve access:")
	fmt.Println(authURL)

	select {
	case code := <-codeCh:
		token, err := config.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("exchange oauth code: %w", err)
		}
		return token, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, errors.New("timed out waiting for oauth callback")
	}
}

func saveToken(path string, token *oauth2.Token) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open token file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(token); err != nil {
		return fmt.Errorf("encode token: %w", err)
	}
	return nil
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
