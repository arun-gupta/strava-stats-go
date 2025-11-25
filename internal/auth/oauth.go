package auth

import (
	"encoding/json"
	"net/http"

	"github.com/arungupta/strava-stats-go/internal/config"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

// StravaEndpoint is the OAuth2 endpoint for Strava.
var StravaEndpoint = oauth2.Endpoint{
	AuthURL:  "https://www.strava.com/oauth/authorize",
	TokenURL: "https://www.strava.com/oauth/token",
}

// Authenticator handles OAuth2 authentication.
type Authenticator struct {
	Config *oauth2.Config
	Store  sessions.Store
}

// NewAuthenticator creates a new Authenticator instance.
func NewAuthenticator(cfg *config.Config) *Authenticator {
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.StravaClientID,
		ClientSecret: cfg.StravaClientSecret,
		RedirectURL:  cfg.StravaCallbackURL,
		Scopes:       []string{"read,activity:read_all"},
		Endpoint:     StravaEndpoint,
	}
	store := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	return &Authenticator{
		Config: oauthConfig,
		Store:  store,
	}
}

// LoginHandler redirects the user to Strava for authentication.
func (a *Authenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	url := a.Config.AuthCodeURL("state", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// CallbackHandler handles the redirect from Strava, exchanges code for token.
func (a *Authenticator) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state != "state" {
		http.Error(w, "State invalid", http.StatusBadRequest)
		return
	}

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

	// Task 2.4: Store the token in session
	session, _ := a.Store.Get(r, "strava-session")
	tokenJson, err := json.Marshal(token)
	if err != nil {
		http.Error(w, "Failed to serialize token", http.StatusInternalServerError)
		return
	}

	session.Values["token"] = string(tokenJson)
	if err := session.Save(r, w); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Authentication Successful!"))
}
