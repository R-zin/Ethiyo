package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R-zin/Ethiyo/internal/auth"
	"github.com/R-zin/Ethiyo/internal/chalo"
	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockChaloService implements chalo.Service for handler unit tests.
type mockChaloService struct {
	getTrackingURLFunc    func(ctx context.Context, busCode string) (string, error)
	getBusTrackingFunc    func(ctx context.Context, busCode string) (*models.BusTrackingInfo, error)
	getBusRouteFunc       func(ctx context.Context, busCode string) (*models.BusRouteInfo, error)
	fetchRouteDetailsFunc func(ctx context.Context, routeURL string) ([]byte, error)
	fetchPublicRouteFunc  func(ctx context.Context, busCode string) (int, error)
}

func (m *mockChaloService) GetTrackingURL(ctx context.Context, busCode string) (string, error) {
	if m.getTrackingURLFunc != nil {
		return m.getTrackingURLFunc(ctx, busCode)
	}
	return "https://chalo.com/app/api/vasudha/track/route-live-info/" + busCode, nil
}

func (m *mockChaloService) GetBusTracking(ctx context.Context, busCode string) (*models.BusTrackingInfo, error) {
	if m.getBusTrackingFunc != nil {
		return m.getBusTrackingFunc(ctx, busCode)
	}
	return &models.BusTrackingInfo{
		BusCode:     busCode,
		TrackingURL: "https://chalo.com/app/api/vasudha/track/route-live-info/" + busCode,
	}, nil
}

func (m *mockChaloService) GetBusRoute(ctx context.Context, busCode string) (*models.BusRouteInfo, error) {
	if m.getBusRouteFunc != nil {
		return m.getBusRouteFunc(ctx, busCode)
	}
	return &models.BusRouteInfo{
		BusCode:     busCode,
		TrackingURL: "https://chalo.com/app/api/vasudha/track/route-live-info/" + busCode,
		RouteURL:    "https://chalo.com/app/api/scheduler_v4/v4/city/routedetailslive?route=" + busCode,
	}, nil
}

func (m *mockChaloService) FetchRouteDetails(ctx context.Context, routeURL string) ([]byte, error) {
	if m.fetchRouteDetailsFunc != nil {
		return m.fetchRouteDetailsFunc(ctx, routeURL)
	}
	return []byte(`{"status":"ok","routeDetails":{"name":"Bus 500"}}`), nil
}

func (m *mockChaloService) FetchPublicRoute(ctx context.Context, busCode string) (int, error) {
	if m.fetchPublicRouteFunc != nil {
		return m.fetchPublicRouteFunc(ctx, busCode)
	}
	return http.StatusOK, nil
}

func setupTestRouter(chaloSvc chalo.Service, authSvc auth.OAuthService) *gin.Engine {
	r := gin.New()
	busHandler := NewBusHandler(chaloSvc)
	healthHandler := NewHealthHandler("path/to/chrome")

	r.GET("/health", healthHandler.Check)
	r.GET("/api/v1/health", healthHandler.Check)

	// V1 routes
	v1 := r.Group("/api/v1")
	{
		v1.GET("/bus/:buscode/track", busHandler.TrackBus)
		v1.GET("/bus/:buscode/route", busHandler.RouteDetails)
		v1.GET("/bus/:buscode/url", busHandler.GetURL)
	}

	// Legacy routes
	r.GET("/trackroute", busHandler.LegacyTrackRoute)
	r.GET("/trackxhr", busHandler.LegacyTrackXHR)
	r.GET("/testroute", busHandler.LegacyTestRoute)
	r.GET("/get_bus_url", busHandler.LegacyGetBusURL)

	if authSvc != nil {
		authHandler := NewAuthHandler(authSvc, false)
		r.GET("/auth/google/login", authHandler.Login)
		r.GET("/auth/google/callback", authHandler.Callback)
		v1.GET("/auth/google/login", authHandler.Login)
		v1.GET("/auth/google/callback", authHandler.Callback)
	}

	return r
}

func TestHealthHandler(t *testing.T) {
	r := setupTestRouter(&mockChaloService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if res["message"] != "ok" || res["status"] != "healthy" {
		t.Errorf("unexpected health response: %v", res)
	}
}

func TestV1TrackBusSuccess(t *testing.T) {
	r := setupTestRouter(&mockChaloService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bus/DL1PC0001/track", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res models.SuccessResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success true")
	}
}

func TestV1TrackBusInvalidCode(t *testing.T) {
	r := setupTestRouter(&mockChaloService{}, nil)
	// Invalid characters: special chars
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bus/bad$$code/track", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}

	var res models.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if res.Error.Code != "INVALID_BUS_CODE" {
		t.Errorf("expected code INVALID_BUS_CODE, got %s", res.Error.Code)
	}
}

// TestV1TrackBusTrimsWhitespace pins that BusTrackingInfo carries the
// validated (trimmed) bus code, not the raw path parameter.
func TestV1TrackBusTrimsWhitespace(t *testing.T) {
	svc := &mockChaloService{
		getBusTrackingFunc: func(ctx context.Context, busCode string) (*models.BusTrackingInfo, error) {
			return &models.BusTrackingInfo{BusCode: busCode, TrackingURL: "https://chalo.com/t/" + busCode}, nil
		},
	}
	r := setupTestRouter(svc, nil)
	// %20BUS500%20 decodes to " BUS500 " in the path segment.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bus/%20BUS500%20/track", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Data models.BusTrackingInfo `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.Data.BusCode != "BUS500" {
		t.Errorf("expected trimmed bus code BUS500, got %q", res.Data.BusCode)
	}
}

func TestV1RouteAndURLSuccess(t *testing.T) {
	r := setupTestRouter(&mockChaloService{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bus/BUS500/route", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("route: expected 200, got %d", w.Code)
	}
	var routeRes struct {
		Data models.BusRouteInfo `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &routeRes); err != nil {
		t.Fatalf("route unmarshal: %v", err)
	}
	if routeRes.Data.TrackingURL == "" || routeRes.Data.RouteURL == "" {
		t.Errorf("route response missing URLs: %+v", routeRes.Data)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/bus/BUS500/url", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("url: expected 200, got %d", w2.Code)
	}
}

func TestHandleErrorDefaultAndCanceled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/boom", func(c *gin.Context) { HandleError(c, errors.New("mystery")) })
	r.GET("/canceled", func(c *gin.Context) { HandleError(c, context.Canceled) })
	r.GET("/nil", func(c *gin.Context) { HandleError(c, nil); c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for unknown error, got %d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/canceled", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for canceled request, got %d", w2.Code)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/nil", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("expected nil error to be a no-op, got %d", w3.Code)
	}
}

func TestHealthBrowserPresence(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rWith := gin.New()
	rWith.GET("/health", NewHealthHandler("/usr/bin/chrome").Check)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	rWith.ServeHTTP(w, req)
	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["browser"] != true {
		t.Errorf("expected browser=true when path configured, got %v", res["browser"])
	}

	rWithout := gin.New()
	rWithout.GET("/health", NewHealthHandler("").Check)
	w2 := httptest.NewRecorder()
	rWithout.ServeHTTP(w2, req)
	var res2 map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &res2)
	if res2["browser"] != false {
		t.Errorf("expected browser=false when no browser found, got %v", res2["browser"])
	}
}

func TestAuthCallbackExchangeFailure(t *testing.T) {
	authSvc := &mockOAuthService{enabled: true, secret: "test-secret-123"}
	r := setupTestRouter(&mockChaloService{}, authSvc)

	state, err := auth.GenerateState("test-secret-123")
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=bad-code&state="+state, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for failed token exchange, got %d", w.Code)
	}
	var res models.ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Error.Code != "TOKEN_EXCHANGE_FAILED" {
		t.Errorf("expected TOKEN_EXCHANGE_FAILED, got %s", res.Error.Code)
	}
}

func TestAuthLoginDisabled(t *testing.T) {
	r := setupTestRouter(&mockChaloService{}, &mockOAuthService{enabled: false})
	req := httptest.NewRequest(http.MethodGet, "/auth/google/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when OAuth disabled, got %d", w.Code)
	}
}

func TestAuthCallbackStateCookieMismatch(t *testing.T) {
	authSvc := &mockOAuthService{enabled: true, secret: "test-secret-123"}
	r := setupTestRouter(&mockChaloService{}, authSvc)

	state, err := auth.GenerateState("test-secret-123")
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}

	otherState, err := auth.GenerateState("test-secret-123")
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=valid-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: otherState})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for cookie/query state mismatch, got %d", w.Code)
	}
}

func TestV1TrackBusNotFound(t *testing.T) {
	svc := &mockChaloService{
		getBusTrackingFunc: func(ctx context.Context, busCode string) (*models.BusTrackingInfo, error) {
			return nil, chalo.ErrBusNotFound
		},
	}
	r := setupTestRouter(svc, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bus/nonexistent/track", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", w.Code)
	}
	var res models.ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Error.Code != "BUS_NOT_FOUND" {
		t.Errorf("expected BUS_NOT_FOUND, got %s", res.Error.Code)
	}
}

func TestV1TrackBusTimeout(t *testing.T) {
	svc := &mockChaloService{
		getBusTrackingFunc: func(ctx context.Context, busCode string) (*models.BusTrackingInfo, error) {
			return nil, chalo.ErrChaloTimeout
		},
	}
	r := setupTestRouter(svc, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bus/slowbus/track", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 Gateway Timeout, got %d", w.Code)
	}
	var res models.ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Error.Code != "CHALO_TIMEOUT" {
		t.Errorf("expected CHALO_TIMEOUT, got %s", res.Error.Code)
	}
}

// TestLegacyEndpointsNoCrash verifies that missing or bad query parameters return
// proper HTTP 400 responses rather than executing log.Fatal() and killing the process.
func TestLegacyEndpointsNoCrash(t *testing.T) {
	r := setupTestRouter(&mockChaloService{}, nil)

	legacyEndpoints := []string{
		"/trackroute?buscode=",
		"/trackxhr?buscode=",
		"/testroute?buscode=",
		"/get_bus_url?buscode=",
		"/trackroute?buscode=../../etc/passwd",
	}

	for _, ep := range legacyEndpoints {
		req := httptest.NewRequest(http.MethodGet, ep, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("endpoint %s expected 400, got %d. Body: %s", ep, w.Code, w.Body.String())
		}
		var errResp models.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
			t.Errorf("endpoint %s returned invalid json: %v", ep, err)
		}
		if errResp.Error.Code != "INVALID_BUS_CODE" {
			t.Errorf("endpoint %s expected INVALID_BUS_CODE, got %s", ep, errResp.Error.Code)
		}
	}
}

func TestLegacyTrackRouteSuccess(t *testing.T) {
	r := setupTestRouter(&mockChaloService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/trackroute?buscode=12345", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestLegacyTrackXHRSuccess(t *testing.T) {
	r := setupTestRouter(&mockChaloService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/trackxhr?buscode=12345", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["message"] != "ok" || res["route"] == "" || res["cookie"] == "" {
		t.Errorf("unexpected legacy response: %v", res)
	}
}

func TestLegacyTestRouteSuccess(t *testing.T) {
	r := setupTestRouter(&mockChaloService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/testroute?buscode=12345", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestLegacyGetBusURLSuccess(t *testing.T) {
	r := setupTestRouter(&mockChaloService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/get_bus_url?buscode=12345", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["result"] == "" {
		t.Errorf("expected result field with tracking URL")
	}
}

// mockOAuthService implements auth.OAuthService for testing.
type mockOAuthService struct {
	enabled bool
	secret  string
}

func (m *mockOAuthService) IsEnabled() bool     { return m.enabled }
func (m *mockOAuthService) StateSecret() string { return m.secret }
func (m *mockOAuthService) AuthCodeURL(state string) (string, error) {
	if !m.enabled {
		return "", auth.ErrOAuthNotConfigured
	}
	return "https://accounts.google.com/o/oauth2/auth?state=" + state, nil
}
func (m *mockOAuthService) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if code == "valid-code" {
		return &oauth2.Token{
			AccessToken: "access-token-123",
			Expiry:      time.Now().Add(1 * time.Hour),
		}, nil
	}
	return nil, errors.New("invalid code")
}
func (m *mockOAuthService) GetUserInfo(ctx context.Context, token *oauth2.Token) (*auth.GoogleUserInfo, error) {
	return &auth.GoogleUserInfo{
		Email: "test@example.com",
		Name:  "Test User",
	}, nil
}

func TestAuthLoginAndCallback(t *testing.T) {
	authSvc := &mockOAuthService{enabled: true, secret: "test-secret-123"}
	r := setupTestRouter(&mockChaloService{}, authSvc)

	// Test Login Redirect
	reqLogin := httptest.NewRequest(http.MethodGet, "/auth/google/login", nil)
	wLogin := httptest.NewRecorder()
	r.ServeHTTP(wLogin, reqLogin)

	if wLogin.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307 redirect, got %d", wLogin.Code)
	}

	cookies := wLogin.Result().Cookies()
	var stateCookie string
	for _, c := range cookies {
		if c.Name == oauthStateCookie {
			stateCookie = c.Value
		}
	}
	if stateCookie == "" {
		t.Fatalf("expected oauth_state cookie to be set")
	}

	// Test Callback with invalid state
	reqBadState := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=valid-code&state=badstate", nil)
	wBadState := httptest.NewRecorder()
	r.ServeHTTP(wBadState, reqBadState)

	if wBadState.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad state, got %d", wBadState.Code)
	}

	// Test Callback with valid state
	reqGood := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=valid-code&state="+stateCookie, nil)
	reqGood.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: stateCookie})
	wGood := httptest.NewRecorder()
	r.ServeHTTP(wGood, reqGood)

	if wGood.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid callback, got %d. Body: %s", wGood.Code, wGood.Body.String())
	}
}
