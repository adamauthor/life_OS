package calendar

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"life_os/internal/app"
	"life_os/internal/domain"
)

const (
	OAuthCallbackPath  = "/oauth/google/callback"
	oauthStateTTL      = 15 * time.Minute
	googleCalendarKind = "google_calendar"
)

type OAuthRepository interface {
	SaveOAuthState(ctx context.Context, state domain.OAuthState) error
	GetOAuthState(ctx context.Context, state string) (domain.OAuthState, error)
	DeleteOAuthState(ctx context.Context, state string) error
	DeleteExpiredOAuthStates(ctx context.Context, now time.Time) error
	SaveGoogleCalendarConnection(ctx context.Context, connection domain.GoogleCalendarConnection) error
	GetGoogleCalendarConnection(ctx context.Context, userID domain.UUID) (domain.GoogleCalendarConnection, error)
	HasGoogleCalendarConnection(ctx context.Context, userID domain.UUID) (bool, error)
	TouchGoogleCalendarConnection(ctx context.Context, userID domain.UUID) error
	DeleteGoogleCalendarConnection(ctx context.Context, userID domain.UUID) error
}

type OAuthService struct {
	repository        OAuthRepository
	config            *oauth2.Config
	defaultCalendarID string
	tokenKey          string
}

type OAuthCallbackResult struct {
	UserID domain.UUID
	ChatID int64
}

func NewOAuthService(repository OAuthRepository, credentialsJSON string, redirectURL string, defaultCalendarID string, tokenEncryptionKey string) (*OAuthService, error) {
	if repository == nil {
		return nil, fmt.Errorf("oauth repository is required")
	}
	if strings.TrimSpace(credentialsJSON) == "" {
		return nil, fmt.Errorf("google credentials JSON is required")
	}
	if strings.TrimSpace(redirectURL) == "" {
		return nil, fmt.Errorf("google oauth redirect URL is required")
	}
	config, err := google.ConfigFromJSON([]byte(credentialsJSON), gcalendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("parse google credentials: %w", err)
	}
	config.RedirectURL = redirectURL
	if strings.TrimSpace(defaultCalendarID) == "" {
		defaultCalendarID = "primary"
	}
	return &OAuthService{
		repository:        repository,
		config:            config,
		defaultCalendarID: defaultCalendarID,
		tokenKey:          tokenEncryptionKey,
	}, nil
}

func (s *OAuthService) BuildConnectURL(ctx context.Context, userID domain.UUID, chatID int64) (string, error) {
	if err := s.repository.DeleteExpiredOAuthStates(ctx, time.Now().UTC()); err != nil {
		return "", err
	}
	state := uuid.NewString()
	if err := s.repository.SaveOAuthState(ctx, domain.OAuthState{
		State:     state,
		UserID:    userID,
		ChatID:    chatID,
		Provider:  googleCalendarKind,
		ExpiresAt: time.Now().UTC().Add(oauthStateTTL),
	}); err != nil {
		return "", err
	}
	return s.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce), nil
}

func (s *OAuthService) HandleCallback(ctx context.Context, stateValue string, code string) (OAuthCallbackResult, error) {
	stateValue = strings.TrimSpace(stateValue)
	code = strings.TrimSpace(code)
	if stateValue == "" {
		return OAuthCallbackResult{}, fmt.Errorf("missing oauth state")
	}
	if code == "" {
		return OAuthCallbackResult{}, fmt.Errorf("missing oauth code")
	}
	state, err := s.repository.GetOAuthState(ctx, stateValue)
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	defer s.repository.DeleteOAuthState(context.Background(), stateValue)
	if state.Provider != googleCalendarKind {
		return OAuthCallbackResult{}, fmt.Errorf("invalid oauth provider")
	}
	if time.Now().UTC().After(state.ExpiresAt) {
		return OAuthCallbackResult{}, fmt.Errorf("oauth state expired")
	}
	token, err := s.config.Exchange(ctx, code)
	if err != nil {
		return OAuthCallbackResult{}, fmt.Errorf("exchange oauth code: %w", err)
	}
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return OAuthCallbackResult{}, fmt.Errorf("marshal oauth token: %w", err)
	}
	storedToken, err := s.encodeTokenJSON(tokenBytes)
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	if err := s.repository.SaveGoogleCalendarConnection(ctx, domain.GoogleCalendarConnection{
		UserID:     state.UserID,
		CalendarID: s.defaultCalendarID,
		TokenJSON:  storedToken,
	}); err != nil {
		return OAuthCallbackResult{}, err
	}
	return OAuthCallbackResult{UserID: state.UserID, ChatID: state.ChatID}, nil
}

func (s *OAuthService) CalendarClientForUser(ctx context.Context, userID domain.UUID) (app.CalendarClient, error) {
	connection, err := s.repository.GetGoogleCalendarConnection(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("calendar is not connected for this user")
	}
	tokenBytes, err := s.decodeTokenJSON(connection.TokenJSON)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(tokenBytes, &token); err != nil {
		return nil, fmt.Errorf("decode stored google token: %w", err)
	}
	httpClient := s.config.Client(ctx, &token)
	service, err := gcalendar.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create google calendar service: %w", err)
	}
	calendarID := connection.CalendarID
	if strings.TrimSpace(calendarID) == "" {
		calendarID = s.defaultCalendarID
	}
	if err := s.repository.TouchGoogleCalendarConnection(ctx, userID); err != nil {
		return nil, err
	}
	return &GoogleClient{calendarID: calendarID, service: service}, nil
}

func (s *OAuthService) IsCalendarConnected(ctx context.Context, userID domain.UUID) bool {
	ok, err := s.repository.HasGoogleCalendarConnection(ctx, userID)
	return err == nil && ok
}

func (s *OAuthService) StatusText(ctx context.Context, userID domain.UUID) (string, error) {
	connection, err := s.repository.GetGoogleCalendarConnection(ctx, userID)
	if err != nil {
		return "Google Calendar не подключен.\n\nПодключить: /connect_calendar", nil
	}
	calendarID := connection.CalendarID
	if strings.TrimSpace(calendarID) == "" {
		calendarID = "primary"
	}
	return fmt.Sprintf("Google Calendar подключен.\nCalendar ID: %s\nConnected: %s\n\nОтключить: /disconnect_calendar", calendarID, connection.ConnectedAt.Format("2006-01-02 15:04")), nil
}

func (s *OAuthService) Disconnect(ctx context.Context, userID domain.UUID) error {
	return s.repository.DeleteGoogleCalendarConnection(ctx, userID)
}

type encryptedTokenJSON struct {
	Encrypted  bool   `json:"encrypted"`
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func (s *OAuthService) encodeTokenJSON(plain []byte) (string, error) {
	if strings.TrimSpace(s.tokenKey) == "" {
		return string(plain), nil
	}
	gcm, err := tokenGCM(s.tokenKey)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("read token nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plain, nil)
	wrapped, err := json.Marshal(encryptedTokenJSON{
		Encrypted:  true,
		Version:    1,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	})
	if err != nil {
		return "", fmt.Errorf("marshal encrypted token: %w", err)
	}
	return string(wrapped), nil
}

func (s *OAuthService) decodeTokenJSON(stored string) ([]byte, error) {
	var wrapped encryptedTokenJSON
	if err := json.Unmarshal([]byte(stored), &wrapped); err == nil && wrapped.Encrypted {
		if strings.TrimSpace(s.tokenKey) == "" {
			return nil, fmt.Errorf("calendar token is encrypted but CALENDAR_TOKEN_ENCRYPTION_KEY is not set")
		}
		gcm, err := tokenGCM(s.tokenKey)
		if err != nil {
			return nil, err
		}
		nonce, err := base64.StdEncoding.DecodeString(wrapped.Nonce)
		if err != nil {
			return nil, fmt.Errorf("decode token nonce: %w", err)
		}
		ciphertext, err := base64.StdEncoding.DecodeString(wrapped.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decode token ciphertext: %w", err)
		}
		plain, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return nil, fmt.Errorf("decrypt calendar token: %w", err)
		}
		return plain, nil
	}
	return []byte(stored), nil
}

func tokenGCM(secret string) (cipher.AEAD, error) {
	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("create token cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create token gcm: %w", err)
	}
	return gcm, nil
}
