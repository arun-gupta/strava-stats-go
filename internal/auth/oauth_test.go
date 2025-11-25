package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arungupta/strava-stats-go/internal/config"
	"golang.org/x/oauth2"
)

func TestLoginHandler(t *testing.T) {
	// 1. Setup
	cfg := &config.Config{
		StravaClientID:     "test-client-id",
		StravaClientSecret: "test-client-secret",
		StravaCallbackURL:  "http://localhost:8080/auth/callback",
		SessionSecret:      "test-secret",
	}
	authenticator := NewAuthenticator(cfg)

	req, err := http.NewRequest("GET", "/auth/login", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authenticator.LoginHandler)

	// 2. Execute
	handler.ServeHTTP(rr, req)

	// 3. Verify
	// Check the status code is what we expect (TemporaryRedirect 307)
	if status := rr.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusTemporaryRedirect)
	}

	// Check the Location header
	location := rr.Header().Get("Location")
	if location == "" {
		t.Errorf("handler returned empty Location header")
	}

	// Check that the location URL looks like a Strava OAuth URL
	expectedBase := "https://www.strava.com/oauth/authorize"
	if !strings.Contains(location, expectedBase) {
		t.Errorf("handler returned unexpected redirect URL: got %v want substring %v",
			location, expectedBase)
	}

	if !strings.Contains(location, "client_id=test-client-id") {
		t.Errorf("redirect URL missing client_id: %v", location)
	}
}

func TestCallbackHandler(t *testing.T) {
	// 1. Setup Mock Token Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != "POST" {
			t.Errorf("Expected POST request for token exchange, got %s", r.Method)
		}
		// Return mock token
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token": "mock-token", "token_type": "Bearer", "expires_in": 3600}`))
	}))
	defer ts.Close()

	// 2. Setup Authenticator
	cfg := &config.Config{
		StravaClientID:     "test-client-id",
		StravaClientSecret: "test-client-secret",
		StravaCallbackURL:  "http://localhost:8080/auth/callback",
		SessionSecret:      "test-secret",
	}
	authenticator := NewAuthenticator(cfg)
	// Override TokenURL to point to our mock server
	authenticator.Config.Endpoint.TokenURL = ts.URL

	// 3. Execute Request
	// Valid request with state and code
	req, err := http.NewRequest("GET", "/auth/callback?state=state&code=valid-code", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authenticator.CallbackHandler)
	handler.ServeHTTP(rr, req)

	// 4. Verify Success
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expectedBody := "Authentication Successful!"
	if rr.Body.String() != expectedBody {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expectedBody)
	}

	// Check if session cookie is set
	cookies := rr.Result().Cookies()
	foundSession := false
	for _, cookie := range cookies {
		if cookie.Name == "strava-session" {
			foundSession = true
			break
		}
	}
	if !foundSession {
		t.Errorf("handler did not set session cookie")
	}

	// 5. Test Invalid State
	reqInvalidState, _ := http.NewRequest("GET", "/auth/callback?state=wrong&code=valid-code", nil)
	rrInvalidState := httptest.NewRecorder()
	handler.ServeHTTP(rrInvalidState, reqInvalidState)
	if rrInvalidState.Code != http.StatusBadRequest {
		t.Errorf("handler should fail on invalid state: got %v", rrInvalidState.Code)
	}

	// 6. Test Missing Code
	reqMissingCode, _ := http.NewRequest("GET", "/auth/callback?state=state", nil)
	rrMissingCode := httptest.NewRecorder()
	handler.ServeHTTP(rrMissingCode, reqMissingCode)
	if rrMissingCode.Code != http.StatusBadRequest {
		t.Errorf("handler should fail on missing code: got %v", rrMissingCode.Code)
	}
}

func TestGetToken_Refresh(t *testing.T) {
	// 1. Setup Mock Token Server for Refresh
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse form data
		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}
		grantType := r.Form.Get("grant_type")
		if grantType != "refresh_token" {
			t.Errorf("Expected grant_type=refresh_token, got %s", grantType)
		}
		refreshToken := r.Form.Get("refresh_token")
		if refreshToken != "old-refresh-token" {
			t.Errorf("Expected refresh_token=old-refresh-token, got %s", refreshToken)
		}

		w.Header().Set("Content-Type", "application/json")
		// Return new token
		w.Write([]byte(`{
			"access_token": "new-access-token",
			"refresh_token": "new-refresh-token",
			"token_type": "Bearer",
			"expires_in": 3600
		}`))
	}))
	defer ts.Close()

	// 2. Setup Authenticator
	cfg := &config.Config{
		StravaClientID:     "test-client-id",
		StravaClientSecret: "test-client-secret",
		StravaCallbackURL:  "http://localhost:8080/auth/callback",
		SessionSecret:      "test-secret",
	}
	authenticator := NewAuthenticator(cfg)
	authenticator.Config.Endpoint.TokenURL = ts.URL

	// 3. Create an expired token
	expiredToken := &oauth2.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}
	tokenJson, _ := json.Marshal(expiredToken)

	// 4. Generate a valid session cookie with this expired token
	// We use a helper handler to bake the cookie
	recorder := httptest.NewRecorder()
	reqSetup, _ := http.NewRequest("GET", "/", nil)
	
	session, _ := authenticator.Store.Get(reqSetup, "strava-session")
	session.Values["token"] = string(tokenJson)
	session.Save(reqSetup, recorder)

	cookie := recorder.Result().Cookies()[0]

	// 5. Call GetToken with the session cookie
	req, _ := http.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()

	newToken, err := authenticator.GetToken(rr, req)
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}

	// 6. Verify
	if newToken.AccessToken != "new-access-token" {
		t.Errorf("Expected new access token, got %s", newToken.AccessToken)
	}
	if newToken.RefreshToken != "new-refresh-token" {
		t.Errorf("Expected new refresh token, got %s", newToken.RefreshToken)
	}

	// Verify session was updated
	// The ResponseRecorder rr should have a Set-Cookie header
	cookies := rr.Result().Cookies()
	foundSession := false
	for _, c := range cookies {
		if c.Name == "strava-session" {
			foundSession = true
			// In a real integration test we might decode this to verify content,
			// but presence of Set-Cookie implies a save occurred because we only save on change.
			break
		}
	}
	if !foundSession {
		t.Errorf("Expected session cookie to be updated (Set-Cookie header missing)")
	}
}
