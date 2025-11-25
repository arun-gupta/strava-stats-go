package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/arungupta/strava-stats-go/internal/auth"
	"github.com/arungupta/strava-stats-go/internal/config"
	"golang.org/x/oauth2"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize OAuth authenticator
	authenticator := auth.NewAuthenticator(cfg)

	port := fmt.Sprintf(":%s", cfg.Port)

	http.HandleFunc("/auth/login", authenticator.LoginHandler)
	http.HandleFunc("/auth/callback", authenticator.CallbackHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		session, _ := authenticator.Store.Get(r, "strava-session")
		var data struct {
			Authenticated bool
			Name          string
		}

		if tokenStr, ok := session.Values["token"].(string); ok && tokenStr != "" {
			data.Authenticated = true
			if name, ok := session.Values["athlete_name"].(string); ok && name != "" {
				data.Name = name
			} else {
				// Self-healing: Name missing, try to fetch it
				var token oauth2.Token
				if err := json.Unmarshal([]byte(tokenStr), &token); err == nil {
					if athlete, err := authenticator.FetchAthlete(r.Context(), &token); err == nil {
						name := strings.TrimSpace(fmt.Sprintf("%s %s", athlete.Firstname, athlete.Lastname))
						if name == "" {
							name = athlete.Username
						}
						if name != "" {
							data.Name = name
							session.Values["athlete_name"] = name
							session.Save(r, w)
						}
					} else {
						log.Printf("Failed to auto-recover athlete name: %v", err)
					}
				}
			}
		}

		tmpl, err := template.ParseFiles("web/templates/index.html")
		if err != nil {
			http.Error(w, "Could not load template", http.StatusInternalServerError)
			log.Printf("Error parsing template: %v", err)
			return
		}
		if err := tmpl.Execute(w, data); err != nil {
			log.Printf("Error executing template: %v", err)
		}
	})
	
	server := &http.Server{
		Addr:         port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Printf("Starting server on http://localhost%s\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
