package handlers

import (
	"net/http"
	"time"

	"github.com/R-zin/Ethiyo/internal/auth"
	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/gin-gonic/gin"
)

const (
	oauthStateCookie = "ethiyo_oauth_state"
	stateMaxAge      = 10 * time.Minute
)

// AuthHandler handles OAuth authentication workflows.
type AuthHandler struct {
	oauthService auth.OAuthService
	isSecure     bool
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(oauthService auth.OAuthService, isSecure bool) *AuthHandler {
	return &AuthHandler{
		oauthService: oauthService,
		isSecure:     isSecure,
	}
}

// Login initiates the Google OAuth authorization flow.
// GET /auth/google/login
func (h *AuthHandler) Login(c *gin.Context) {
	if !h.oauthService.IsEnabled() {
		HandleError(c, auth.ErrOAuthNotConfigured)
		return
	}

	state, err := auth.GenerateState(h.oauthService.StateSecret())
	if err != nil {
		HandleError(c, err)
		return
	}

	// Store state in an HTTP-only, secure cookie for CSRF prevention
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		oauthStateCookie,
		state,
		int(stateMaxAge.Seconds()),
		"/",
		"",
		h.isSecure,
		true, // HttpOnly
	)

	authURL, err := h.oauthService.AuthCodeURL(state)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// Callback handles the OAuth redirect response from Google.
// GET /auth/google/callback
func (h *AuthHandler) Callback(c *gin.Context) {
	if !h.oauthService.IsEnabled() {
		HandleError(c, auth.ErrOAuthNotConfigured)
		return
	}

	state := c.Query("state")
	code := c.Query("code")

	if code == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse("MISSING_AUTH_CODE", "authorization code is required"))
		return
	}

	if state == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse("MISSING_OAUTH_STATE", "oauth state parameter is required"))
		return
	}

	// 1. Verify cryptographic state signature and expiration
	if err := auth.ValidateState(state, h.oauthService.StateSecret(), stateMaxAge); err != nil {
		HandleError(c, err)
		return
	}

	// 2. Verify state matches client's state cookie if present
	if cookieState, err := c.Cookie(oauthStateCookie); err == nil && cookieState != "" {
		if cookieState != state {
			c.JSON(http.StatusBadRequest, models.NewErrorResponse("INVALID_OAUTH_STATE", "oauth state mismatch"))
			return
		}
	}

	// Clear the state cookie
	c.SetCookie(oauthStateCookie, "", -1, "/", "", h.isSecure, true)

	// 3. Exchange code for access token using request context
	token, err := h.oauthService.Exchange(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.NewErrorResponse("TOKEN_EXCHANGE_FAILED", "Failed to exchange token with provider"))
		return
	}

	// Fetch user details if possible
	var userInfo *auth.GoogleUserInfo
	if info, err := h.oauthService.GetUserInfo(c.Request.Context(), token); err == nil {
		userInfo = info
	}

	// Return clean response (maintains legacy fields while omitting sensitive refresh token)
	c.JSON(http.StatusOK, gin.H{
		"message":      "ok",
		"access_token": token.AccessToken,
		"expiry":       token.Expiry,
		"user":         userInfo,
	})
}
