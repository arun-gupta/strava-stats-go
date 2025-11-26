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

// setupStateForTest creates a session with a valid OAuth state for testing
func setupStateForTest(t *testing.T, authenticator *Authenticator) (string, *http.Cookie) {
	loginReq, _ := http.NewRequest("GET", "/auth/login", nil)
	loginRR := httptest.NewRecorder()
	loginHandler := http.HandlerFunc(authenticator.LoginHandler)
	loginHandler.ServeHTTP(loginRR, loginReq)

	loginCookies := loginRR.Result().Cookies()
	if len(loginCookies) == 0 {
		t.Fatal("Login handler did not set session cookie")
	}

	reqWithLoginCookie := &http.Request{Header: http.Header{"Cookie": loginRR.Header()["Set-Cookie"]}}
	session, _ := authenticator.Store.Get(reqWithLoginCookie, "strava-session")
	state, ok := session.Values["oauth_state"].(string)
	if !ok || state == "" {
		t.Fatal("State not found in session after login")
	}

	return state, loginCookies[0]
}

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
		// Return mock token with athlete info
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"access_token": "mock-token", 
			"token_type": "Bearer", 
			"expires_in": 3600,
			"athlete": {
				"firstname": "John",
				"lastname": "Doe"
			}
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
	// Override TokenURL to point to our mock server
	authenticator.Config.Endpoint.TokenURL = ts.URL

	// 3. First, call LoginHandler to generate and store state in session
	loginReq, _ := http.NewRequest("GET", "/auth/login", nil)
	loginRR := httptest.NewRecorder()
	loginHandler := http.HandlerFunc(authenticator.LoginHandler)
	loginHandler.ServeHTTP(loginRR, loginReq)

	// Extract the state from the session cookie
	loginCookies := loginRR.Result().Cookies()
	if len(loginCookies) == 0 {
		t.Fatal("Login handler did not set session cookie")
	}

	// Get the state from the session
	reqWithLoginCookie := &http.Request{Header: http.Header{"Cookie": loginRR.Header()["Set-Cookie"]}}
	session, _ := authenticator.Store.Get(reqWithLoginCookie, "strava-session")
	state, ok := session.Values["oauth_state"].(string)
	if !ok || state == "" {
		t.Fatal("State not found in session after login")
	}

	// 4. Execute Callback Request with the correct state
	req, err := http.NewRequest("GET", "/auth/callback?state="+state+"&code=valid-code", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Add the session cookie to the callback request
	req.AddCookie(loginCookies[0])

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authenticator.CallbackHandler)
	handler.ServeHTTP(rr, req)

	// 4. Verify Success
	if status := rr.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusTemporaryRedirect)
	}

	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("handler returned wrong location: got %v want /", loc)
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

	// Check if athlete name is stored in session
	// Parse the cookie from the response and create a new request with it
	reqWithCookie := &http.Request{Header: http.Header{"Cookie": rr.Header()["Set-Cookie"]}}
	callbackSession, _ := authenticator.Store.Get(reqWithCookie, "strava-session")
	if name, ok := callbackSession.Values["athlete_name"]; !ok || name != "John Doe" {
		t.Errorf("session does not contain correct athlete name: got %v want 'John Doe'", name)
	}

	// 5. Test Invalid State (state mismatch)
	reqInvalidState, _ := http.NewRequest("GET", "/auth/callback?state=wrong-state&code=valid-code", nil)
	reqInvalidState.AddCookie(loginCookies[0]) // Use same session but wrong state
	rrInvalidState := httptest.NewRecorder()
	handler.ServeHTTP(rrInvalidState, reqInvalidState)
	if rrInvalidState.Code != http.StatusBadRequest {
		t.Errorf("handler should fail on invalid state: got %v", rrInvalidState.Code)
	}

	// 6. Test Missing Code
	reqMissingCode, _ := http.NewRequest("GET", "/auth/callback?state="+state, nil)
	reqMissingCode.AddCookie(loginCookies[0])
	rrMissingCode := httptest.NewRecorder()
	handler.ServeHTTP(rrMissingCode, reqMissingCode)
	if rrMissingCode.Code != http.StatusBadRequest {
		t.Errorf("handler should fail on missing code: got %v", rrMissingCode.Code)
	}

	// 7. Test Missing State Parameter
	reqMissingState, _ := http.NewRequest("GET", "/auth/callback?code=valid-code", nil)
	reqMissingState.AddCookie(loginCookies[0])
	rrMissingState := httptest.NewRecorder()
	handler.ServeHTTP(rrMissingState, reqMissingState)
	if rrMissingState.Code != http.StatusBadRequest {
		t.Errorf("handler should fail on missing state parameter: got %v", rrMissingState.Code)
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

func TestCallbackHandler_UsernameFallback(t *testing.T) {
	// 1. Setup Mock Token Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"access_token": "mock-token", 
			"token_type": "Bearer", 
			"expires_in": 3600,
			"athlete": {
				"username": "strava_user_123"
			}
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

	// 3. Setup state in session
	state, cookie := setupStateForTest(t, authenticator)

	// 4. Execute Request
	req, err := http.NewRequest("GET", "/auth/callback?state="+state+"&code=valid-code", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(cookie)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authenticator.CallbackHandler)
	handler.ServeHTTP(rr, req)

	// 4. Verify
	if status := rr.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusTemporaryRedirect)
	}

	// Check if athlete name is stored in session as username
	reqWithCookie := &http.Request{Header: http.Header{"Cookie": rr.Header()["Set-Cookie"]}}
	session, _ := authenticator.Store.Get(reqWithCookie, "strava-session")
	if name, ok := session.Values["athlete_name"]; !ok || name != "strava_user_123" {
		t.Errorf("session does not contain correct fallback username: got %v want 'strava_user_123'", name)
	}
}

func TestCallbackHandler_FetchFallback(t *testing.T) {
	// 1. Setup Mock Server (handles both Token and API requests)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			// Token response WITHOUT athlete
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"access_token": "mock-token", 
				"token_type": "Bearer", 
				"expires_in": 3600
			}`))
			return
		}
		if r.URL.Path == "/athlete" {
			// API response
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"id": 123,
				"firstname": "Fetched",
				"lastname": "User"
			}`))
			return
		}
		http.NotFound(w, r)
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
	authenticator.Config.Endpoint.TokenURL = ts.URL + "/oauth/token"
	authenticator.StravaAPIURL = ts.URL // Mock the API base URL

	// 3. Setup state in session
	state, cookie := setupStateForTest(t, authenticator)

	// 4. Execute Request
	req, err := http.NewRequest("GET", "/auth/callback?state="+state+"&code=valid-code", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(cookie)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authenticator.CallbackHandler)
	handler.ServeHTTP(rr, req)

	// 4. Verify
	if status := rr.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusTemporaryRedirect)
	}

	// Check if athlete name is stored in session
	reqWithCookie := &http.Request{Header: http.Header{"Cookie": rr.Header()["Set-Cookie"]}}
	session, _ := authenticator.Store.Get(reqWithCookie, "strava-session")
	if name, ok := session.Values["athlete_name"]; !ok || name != "Fetched User" {
		t.Errorf("session does not contain fetched athlete name: got %v want 'Fetched User'", name)
	}
}

func TestCallbackHandler_ProfileExtraction(t *testing.T) {
	// 1. Setup Mock Token Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"access_token": "mock-token", 
			"token_type": "Bearer", 
			"expires_in": 3600,
			"athlete": {
				"firstname": "Jane",
				"lastname": "Doe",
				"profile": "http://example.com/jane.jpg"
			}
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

	// 3. Setup state in session
	state, cookie := setupStateForTest(t, authenticator)

	// 4. Execute Request
	req, _ := http.NewRequest("GET", "/auth/callback?state="+state+"&code=valid-code", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authenticator.CallbackHandler)
	handler.ServeHTTP(rr, req)

	// 4. Verify
	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusTemporaryRedirect)
	}

	reqWithCookie := &http.Request{Header: http.Header{"Cookie": rr.Header()["Set-Cookie"]}}
	session, _ := authenticator.Store.Get(reqWithCookie, "strava-session")
	
	if name, ok := session.Values["athlete_name"]; !ok || name != "Jane Doe" {
		t.Errorf("session does not contain correct athlete name: got %v", name)
	}
	
	if profile, ok := session.Values["athlete_profile"]; !ok || profile != "http://example.com/jane.jpg" {
		t.Errorf("session does not contain correct athlete profile: got %v", profile)
	}
}

func TestLogoutHandler(t *testing.T) {
	// 1. Setup Authenticator
	cfg := &config.Config{
		SessionSecret: "test-secret",
	}
	authenticator := NewAuthenticator(cfg)

	// 2. Create a request
	req, _ := http.NewRequest("GET", "/auth/logout", nil)
	rr := httptest.NewRecorder()
	
	handler := http.HandlerFunc(authenticator.LogoutHandler)
	handler.ServeHTTP(rr, req)

	// 3. Verify
	if rr.Code != http.StatusFound {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusFound)
	}

	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("handler returned wrong location: got %v want /", loc)
	}

	// Verify cookie deletion (MaxAge < 0)
	cookies := rr.Result().Cookies()
	foundSession := false
	for _, c := range cookies {
		if c.Name == "strava-session" {
			foundSession = true
			if c.MaxAge >= 0 {
				t.Errorf("session cookie should have negative MaxAge to expire it, got %d", c.MaxAge)
			}
			break
		}
	}
	if !foundSession {
		t.Errorf("handler did not set session cookie (to delete it)")
	}
}
