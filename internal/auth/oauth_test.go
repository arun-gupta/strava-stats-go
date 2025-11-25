package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arungupta/strava-stats-go/internal/config"
)

func TestLoginHandler(t *testing.T) {
	// 1. Setup
	cfg := &config.Config{
		StravaClientID:     "test-client-id",
		StravaClientSecret: "test-client-secret",
		StravaCallbackURL:  "http://localhost:8080/auth/callback",
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
