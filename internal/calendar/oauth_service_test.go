package calendar

import (
	"context"
	"strings"
	"testing"
	"time"

	"life_os/internal/domain"
)

type fakeOAuthRepository struct {
	state domain.OAuthState
}

func (r *fakeOAuthRepository) SaveOAuthState(_ context.Context, state domain.OAuthState) error {
	r.state = state
	return nil
}

func (r *fakeOAuthRepository) GetOAuthState(_ context.Context, _ string) (domain.OAuthState, error) {
	return r.state, nil
}

func (r *fakeOAuthRepository) DeleteOAuthState(_ context.Context, _ string) error {
	return nil
}

func (r *fakeOAuthRepository) DeleteExpiredOAuthStates(_ context.Context, _ time.Time) error {
	return nil
}

func (r *fakeOAuthRepository) SaveGoogleCalendarConnection(_ context.Context, _ domain.GoogleCalendarConnection) error {
	return nil
}

func (r *fakeOAuthRepository) UpdateGoogleCalendarToken(_ context.Context, _ domain.UUID, _ string) error {
	return nil
}

func (r *fakeOAuthRepository) GetGoogleCalendarConnection(_ context.Context, _ domain.UUID) (domain.GoogleCalendarConnection, error) {
	return domain.GoogleCalendarConnection{}, nil
}

func (r *fakeOAuthRepository) HasGoogleCalendarConnection(_ context.Context, _ domain.UUID) (bool, error) {
	return false, nil
}

func (r *fakeOAuthRepository) TouchGoogleCalendarConnection(_ context.Context, _ domain.UUID) error {
	return nil
}

func (r *fakeOAuthRepository) DeleteGoogleCalendarConnection(_ context.Context, _ domain.UUID) error {
	return nil
}

func TestBuildConnectURLStoresState(t *testing.T) {
	repository := &fakeOAuthRepository{}
	service, err := NewOAuthService(repository, testGoogleCredentialsJSON, "http://localhost:8080/oauth/google/callback", "primary", "test-secret")
	if err != nil {
		t.Fatalf("NewOAuthService returned error: %v", err)
	}
	userID := domain.UserIDFromTelegram(123)

	url, err := service.BuildConnectURL(context.Background(), userID, 456)
	if err != nil {
		t.Fatalf("BuildConnectURL returned error: %v", err)
	}
	if !strings.Contains(url, "accounts.google.com") {
		t.Fatalf("auth url = %q, want google auth url", url)
	}
	if repository.state.UserID != userID {
		t.Fatalf("state.UserID = %s, want %s", repository.state.UserID, userID)
	}
	if repository.state.ChatID != 456 {
		t.Fatalf("state.ChatID = %d, want 456", repository.state.ChatID)
	}
	if repository.state.ExpiresAt.Before(time.Now().UTC()) {
		t.Fatal("state should expire in the future")
	}
}

func TestTokenEncryptionRoundTrip(t *testing.T) {
	service, err := NewOAuthService(&fakeOAuthRepository{}, testGoogleCredentialsJSON, "http://localhost:8080/oauth/google/callback", "primary", "test-secret")
	if err != nil {
		t.Fatalf("NewOAuthService returned error: %v", err)
	}

	stored, err := service.encodeTokenJSON([]byte(`{"access_token":"secret"}`))
	if err != nil {
		t.Fatalf("encodeTokenJSON returned error: %v", err)
	}
	if !strings.Contains(stored, `"encrypted":true`) {
		t.Fatalf("stored token = %q, want encrypted wrapper", stored)
	}
	plain, err := service.decodeTokenJSON(stored)
	if err != nil {
		t.Fatalf("decodeTokenJSON returned error: %v", err)
	}
	if string(plain) != `{"access_token":"secret"}` {
		t.Fatalf("plain token = %q", plain)
	}
}

const testGoogleCredentialsJSON = `{
  "web": {
    "client_id": "test-client-id",
    "client_secret": "test-client-secret",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "redirect_uris": ["http://localhost:8080/oauth/google/callback"]
  }
}`
