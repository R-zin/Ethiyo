package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/R-zin/Ethiyo/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	// ErrOAuthNotConfigured is returned if OAuth endpoints are accessed without proper credentials.
	ErrOAuthNotConfigured = errors.New("google oauth is not configured; set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URL")

	// ErrFailedTokenExchange indicates failure during authorization code exchange.
	ErrFailedTokenExchange = errors.New("failed to exchange authorization code for token")
)

// GoogleUserInfo represents basic profile details returned by Google's userinfo endpoint.
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// TokenResult contains safe token output and optional user profile.
type TokenResult struct {
	AccessToken  string          `json:"access_token,omitempty"`
	RefreshToken string          `json:"refresh_token,omitempty"`
	Expiry       time.Time       `json:"expiry"`
	User         *GoogleUserInfo `json:"user,omitempty"`
}

// OAuthService defines operations for Google OAuth authentication.
type OAuthService interface {
	AuthCodeURL(state string) (string, error)
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error)
	StateSecret() string
	IsEnabled() bool
}

// GoogleService implements OAuthService.
type GoogleService struct {
	cfg         *config.Config
	oauthConfig *oauth2.Config
}

// NewGoogleService initializes a new Google OAuth service.
func NewGoogleService(cfg *config.Config) *GoogleService {
	var oauthConfig *oauth2.Config
	if cfg.IsOAuthEnabled() {
		oauthConfig = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		}
	}

	return &GoogleService{
		cfg:         cfg,
		oauthConfig: oauthConfig,
	}
}

// IsEnabled returns true if OAuth configuration is complete.
func (s *GoogleService) IsEnabled() bool {
	return s.oauthConfig != nil
}

// StateSecret returns the configured or generated secret for signing state tokens.
func (s *GoogleService) StateSecret() string {
	return s.cfg.OAuthStateSecret
}

// AuthCodeURL constructs the Google OAuth login URL with offline access and prompt options.
func (s *GoogleService) AuthCodeURL(state string) (string, error) {
	if !s.IsEnabled() {
		return "", ErrOAuthNotConfigured
	}
	return s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

// Exchange exchanges the incoming authorization code for OAuth tokens.
func (s *GoogleService) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if !s.IsEnabled() {
		return nil, ErrOAuthNotConfigured
	}
	token, err := s.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFailedTokenExchange, err)
	}
	return token, nil
}

// GetUserInfo fetches the authenticated user's profile info from Google.
func (s *GoogleService) GetUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	if !s.IsEnabled() {
		return nil, ErrOAuthNotConfigured
	}

	client := s.oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo returned status %d", resp.StatusCode)
	}

	var user GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info response: %w", err)
	}

	return &user, nil
}
