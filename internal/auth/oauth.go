package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/arungupta/strava-stats-go/internal/config"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

// StravaEndpoint is the OAuth2 endpoint for Strava.
var StravaEndpoint = oauth2.Endpoint{
	AuthURL:  "https://www.strava.com/oauth/authorize",
	TokenURL: "https://www.strava.com/oauth/token",
}

// Athlete represents the Strava athlete profile.
type Athlete struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Profile   string `json:"profile"`
}

// Authenticator handles OAuth2 authentication.
type Authenticator struct {
	Config       *oauth2.Config
	Store        sessions.Store
	StravaAPIURL string
}

// NewAuthenticator creates a new Authenticator instance.
func NewAuthenticator(cfg *config.Config) *Authenticator {
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.StravaClientID,
		ClientSecret: cfg.StravaClientSecret,
		RedirectURL:  cfg.StravaCallbackURL,
		Scopes:       []string{"read", "activity:read_all"},
		Endpoint:     StravaEndpoint,
	}
	store := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	return &Authenticator{
		Config:       oauthConfig,
		Store:        store,
		StravaAPIURL: "https://www.strava.com/api/v3",
	}
}

// LoginHandler redirects the user to Strava for authentication.
func (a *Authenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Generate a cryptographically random state value for CSRF protection
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		http.Error(w, "Failed to generate state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	state := base64.URLEncoding.EncodeToString(stateBytes)

	// Store the state in the session for verification in the callback
	session, _ := a.Store.Get(r, "strava-session")
	session.Values["oauth_state"] = state
	if err := session.Save(r, w); err != nil {
		http.Error(w, "Failed to save session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Strava requires comma-separated scopes, not space-separated (which the Go OAuth2 library uses)
	// Build the authorization URL manually to ensure proper formatting
	authURL, err := url.Parse(a.Config.Endpoint.AuthURL)
	if err != nil {
		http.Error(w, "Failed to parse auth URL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	params := url.Values{}
	params.Set("client_id", a.Config.ClientID)
	params.Set("redirect_uri", a.Config.RedirectURL)
	params.Set("response_type", "code")
	params.Set("state", state)
	params.Set("scope", strings.Join(a.Config.Scopes, ",")) // Comma-separated for Strava
	params.Set("approval_prompt", "force") // Request offline access (refresh token)

	authURL.RawQuery = params.Encode()
	http.Redirect(w, r, authURL.String(), http.StatusTemporaryRedirect)
}

// LogoutHandler logs the user out by clearing the session.
func (a *Authenticator) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := a.Store.Get(r, "strava-session")
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// CallbackHandler handles the redirect from Strava, exchanges code for token.
func (a *Authenticator) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve and verify the state parameter
	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "State parameter missing", http.StatusBadRequest)
		return
	}

	// Get the state from the session
	session, _ := a.Store.Get(r, "strava-session")
	sessionState, ok := session.Values["oauth_state"].(string)
	if !ok || sessionState == "" {
		http.Error(w, "State not found in session", http.StatusBadRequest)
		return
	}

	// Verify the state matches (CSRF protection)
	if state != sessionState {
		http.Error(w, "State mismatch - possible CSRF attack", http.StatusBadRequest)
		return
	}

	// Clear the state from session after verification
	delete(session.Values, "oauth_state")

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for an access token
	token, err := a.Config.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Task 2.4: Store the token in session (session already retrieved above)
	tokenJson, err := json.Marshal(token)
	if err != nil {
		http.Error(w, "Failed to serialize token", http.StatusInternalServerError)
		return
	}

	session.Values["token"] = string(tokenJson)

	// Extract athlete info
	var displayName string
	var profileURL string

	if athlete := token.Extra("athlete"); athlete != nil {
		if athleteMap, ok := athlete.(map[string]interface{}); ok {
			firstname, _ := athleteMap["firstname"].(string)
			lastname, _ := athleteMap["lastname"].(string)
			username, _ := athleteMap["username"].(string)
			profileURL, _ = athleteMap["profile"].(string)

			displayName = strings.TrimSpace(fmt.Sprintf("%s %s", firstname, lastname))
			if displayName == "" {
				displayName = username
			}
		}
	}

	// Fallback: Fetch if missing
	if displayName == "" || profileURL == "" {
		log.Println("Athlete data missing or incomplete in token response, fetching from API...")
		if fetchedAthlete, err := a.FetchAthlete(r.Context(), token); err == nil {
			if displayName == "" {
				displayName = strings.TrimSpace(fmt.Sprintf("%s %s", fetchedAthlete.Firstname, fetchedAthlete.Lastname))
				if displayName == "" {
					displayName = fetchedAthlete.Username
				}
			}
			if profileURL == "" {
				profileURL = fetchedAthlete.Profile
			}
		} else {
			log.Printf("Failed to fetch athlete: %v", err)
		}
	}

	if displayName != "" {
		session.Values["athlete_name"] = displayName
	} else {
		log.Println("Warning: Could not determine athlete name")
	}

	if profileURL != "" {
		session.Values["athlete_profile"] = profileURL
	}

	if err := session.Save(r, w); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// GetToken retrieves the token from the session, refreshing it if necessary.
func (a *Authenticator) GetToken(w http.ResponseWriter, r *http.Request) (*oauth2.Token, error) {
	session, _ := a.Store.Get(r, "strava-session")
	val, ok := session.Values["token"]
	if !ok {
		return nil, fmt.Errorf("no token found in session")
	}

	tokenStr, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("invalid token format in session")
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(tokenStr), &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	// Create a TokenSource that will automatically refresh the token if it's expired.
	// Note: We must use a.Config.TokenSource to get the refresh behavior.
	src := a.Config.TokenSource(r.Context(), &token)
	newToken, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get valid token: %w", err)
	}

	// If the token has changed (refreshed), update the session.
	if newToken.AccessToken != token.AccessToken || newToken.RefreshToken != token.RefreshToken {
		newTokenJson, err := json.Marshal(newToken)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal new token: %w", err)
		}
		session.Values["token"] = string(newTokenJson)
		if err := session.Save(r, w); err != nil {
			return nil, fmt.Errorf("failed to save refreshed token to session: %w", err)
		}
	}

	return newToken, nil
}

// FetchAthlete retrieves the authenticated athlete's profile from Strava API.
func (a *Authenticator) FetchAthlete(ctx context.Context, token *oauth2.Token) (*Athlete, error) {
	client := a.Config.Client(ctx, token)
	resp, err := client.Get(fmt.Sprintf("%s/athlete", a.StravaAPIURL))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch athlete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch athlete, status: %s", resp.Status)
	}

	var athlete Athlete
	if err := json.NewDecoder(resp.Body).Decode(&athlete); err != nil {
		return nil, fmt.Errorf("failed to decode athlete response: %w", err)
	}
	return &athlete, nil
}
