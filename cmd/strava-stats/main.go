package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/arungupta/strava-stats-go/internal/api"
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

	// Initialize Strava API client
	stravaClient := api.NewClient(authenticator.StravaAPIURL, authenticator.Config)

	port := fmt.Sprintf(":%s", cfg.Port)

	http.HandleFunc("/auth/login", authenticator.LoginHandler)
	http.HandleFunc("/auth/logout", authenticator.LogoutHandler)
	http.HandleFunc("/auth/callback", authenticator.CallbackHandler)
	
	// API endpoint for fetching activities
	http.HandleFunc("/api/activities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Get token from session (this automatically refreshes if expired)
		token, err := authenticator.GetToken(w, r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized: " + err.Error()})
			return
		}

		// Fetch activities from Strava API
		activities, err := stravaClient.FetchActivities(r.Context(), token, nil)
		if err != nil {
			// Check if it's an API error with specific status code
			if apiErr, ok := err.(*api.APIError); ok {
				// Handle rate limiting (429)
				if apiErr.IsRateLimit() {
					log.Printf("Rate limit exceeded: %v, retry after: %v", apiErr.Message, apiErr.RetryAfter)
					w.WriteHeader(http.StatusTooManyRequests)
					response := map[string]interface{}{
						"error":      "Rate limit exceeded. Please try again later.",
						"message":    apiErr.Message,
						"retry_after": int(apiErr.RetryAfter.Seconds()),
					}
					json.NewEncoder(w).Encode(response)
					return
				}

				// Handle unauthorized (401) - try to refresh token and retry once
				if apiErr.IsUnauthorized() {
					log.Printf("Unauthorized error, attempting token refresh: %v", apiErr.Message)
					// Get a fresh token (this will attempt refresh)
					newToken, refreshErr := authenticator.GetToken(w, r)
					if refreshErr != nil {
						log.Printf("Token refresh failed: %v", refreshErr)
						w.WriteHeader(http.StatusUnauthorized)
						json.NewEncoder(w).Encode(map[string]string{
							"error": "Unauthorized: token refresh failed. Please log in again.",
						})
						return
					}

					// Retry the request with the refreshed token
					activities, err = stravaClient.FetchActivities(r.Context(), newToken, nil)
					if err != nil {
						log.Printf("Failed to fetch activities after token refresh: %v", err)
						// Check if it's still an API error
						if retryApiErr, ok := err.(*api.APIError); ok {
							if retryApiErr.IsUnauthorized() {
								w.WriteHeader(http.StatusUnauthorized)
								json.NewEncoder(w).Encode(map[string]string{
									"error": "Unauthorized: please log in again.",
								})
								return
							}
							// Handle other API errors from retry
							if retryApiErr.IsServerError() {
								w.WriteHeader(http.StatusBadGateway)
								json.NewEncoder(w).Encode(map[string]string{
									"error": "Strava API is temporarily unavailable. Please try again later.",
								})
								return
							}
							w.WriteHeader(retryApiErr.StatusCode)
							json.NewEncoder(w).Encode(map[string]string{
								"error": retryApiErr.Message,
							})
							return
						}
						// Generic error from retry
						log.Printf("Failed to fetch activities after token refresh: %v", err)
						w.WriteHeader(http.StatusInternalServerError)
						json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch activities: " + err.Error()})
						return
					}
					// Success after refresh, continue with activities
					log.Printf("Successfully fetched activities after token refresh")
				} else if apiErr.IsServerError() {
					// Handle server errors (5xx)
					log.Printf("Strava API server error: %v", apiErr.Message)
					w.WriteHeader(http.StatusBadGateway)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "Strava API is temporarily unavailable. Please try again later.",
						"message": apiErr.Message,
					})
					return
				} else {
					// Other API errors
					log.Printf("API error: %v", apiErr)
					w.WriteHeader(apiErr.StatusCode)
					json.NewEncoder(w).Encode(map[string]string{
						"error": apiErr.Message,
					})
					return
				}
			} else {
				// Generic error (not an APIError)
				log.Printf("Failed to fetch activities: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch activities: " + err.Error()})
				return
			}
		}

		// Normalize activities (last 7 days by default)
		normalized := api.NormalizeActivities(activities, nil)

		// Calculate summary statistics
		var totalMovingTime int
		var earliestDateStr, latestDateStr string
		
		for _, activity := range normalized {
			totalMovingTime += activity.MovingTime
			
			// Track date range using LocalDateStr (YYYY-MM-DD) for consistency
			// This ensures the date range matches exactly what's in the activities
			if activity.LocalDateStr != "" {
				if earliestDateStr == "" || activity.LocalDateStr < earliestDateStr {
					earliestDateStr = activity.LocalDateStr
				}
				if latestDateStr == "" || activity.LocalDateStr > latestDateStr {
					latestDateStr = activity.LocalDateStr
				}
			}
		}

		// Format date range for display
		var dateRange string
		var startDateStr, endDateStr string
		if earliestDateStr != "" && latestDateStr != "" {
			// Parse the date strings to format for display
			earliestDate, err1 := time.Parse("2006-01-02", earliestDateStr)
			latestDate, err2 := time.Parse("2006-01-02", latestDateStr)
			if err1 == nil && err2 == nil {
				dateRange = fmt.Sprintf("%s - %s", 
					earliestDate.Format("Jan 2"), 
					latestDate.Format("Jan 2"))
			} else {
				dateRange = fmt.Sprintf("%s - %s", earliestDateStr, latestDateStr)
			}
			// Send date strings in YYYY-MM-DD format for frontend use
			startDateStr = earliestDateStr
			endDateStr = latestDateStr
		} else {
			dateRange = "No activities"
		}

		// Format total moving time
		movingTimeFormatted := api.FormatDuration(totalMovingTime)

		// Prepare response
		response := map[string]interface{}{
			"dateRange":      dateRange,
			"startDate":      startDateStr,
			"endDate":        endDateStr,
			"totalActivities": len(normalized),
			"totalMovingTime": movingTimeFormatted,
			"activities":     normalized,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	})

	// API endpoint for running statistics
	http.HandleFunc("/api/running-stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get token from session
		token, err := authenticator.GetToken(w, r)
		if err != nil {
			log.Printf("Running stats: unauthorized: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized: " + err.Error()})
			return
		}
		
		log.Printf("Running stats: fetching activities for user")

		// Use the same date range logic as the main activities endpoint
		// Fetch activities without date filtering - let normalization handle the date range
		// This ensures consistency with the main endpoint
		activities, err := stravaClient.FetchActivities(r.Context(), token, nil)
		if err != nil {
			if apiErr, ok := err.(*api.APIError); ok {
				if apiErr.IsRateLimit() {
					w.WriteHeader(http.StatusTooManyRequests)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error":      "Rate limit exceeded. Please try again later.",
						"message":    apiErr.Message,
						"retry_after": int(apiErr.RetryAfter.Seconds()),
					})
					return
				}
					if apiErr.IsUnauthorized() {
						// Try to refresh token and retry once
						newToken, getErr := authenticator.GetToken(w, r)
						if getErr == nil {
							activities, err = stravaClient.FetchActivities(r.Context(), newToken, nil)
						if err != nil {
							w.WriteHeader(http.StatusUnauthorized)
							json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized: token refresh failed"})
							return
						}
					} else {
						w.WriteHeader(http.StatusUnauthorized)
						json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized: " + apiErr.Message})
						return
					}
				} else if apiErr.IsServerError() {
					w.WriteHeader(http.StatusBadGateway)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "Strava API is temporarily unavailable. Please try again later.",
					})
					return
				} else {
					w.WriteHeader(apiErr.StatusCode)
					json.NewEncoder(w).Encode(map[string]string{"error": apiErr.Message})
					return
				}
			} else {
				log.Printf("Running stats: failed to fetch activities: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch activities: " + err.Error()})
				return
			}
		}
		
		log.Printf("Running stats: fetched %d activities", len(activities))

		// Normalize activities using the same date range as the main endpoint (default 7 days)
		normalized := api.NormalizeActivities(activities, nil)
		log.Printf("Running stats: normalized to %d activities", len(normalized))

		// Calculate running statistics
		stats := api.CalculateRunningStats(normalized)
		prs := api.CalculatePersonalRecords(normalized)
		
		// Generate distance histogram (use miles for now, can be made configurable)
		histogram := api.GenerateDistanceHistogram(normalized, true) // true = use miles

		// Prepare response - always return valid structure even if empty
		response := map[string]interface{}{
			"stats":     stats,
			"prs":       prs,
			"histogram": histogram,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Running stats: failed to encode response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to encode response: " + err.Error()})
			return
		}
		
		log.Printf("Running stats: successfully returned stats: %d total runs", stats.TotalRuns)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Don't handle API routes - they should be handled by their specific handlers
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		
		session, _ := authenticator.Store.Get(r, "strava-session")
		var data struct {
			Authenticated bool
			Name          string
			ProfileURL    string
		}

		if tokenStr, ok := session.Values["token"].(string); ok && tokenStr != "" {
			data.Authenticated = true
			if name, ok := session.Values["athlete_name"].(string); ok && name != "" {
				data.Name = name
			}
			if profile, ok := session.Values["athlete_profile"].(string); ok && profile != "" {
				data.ProfileURL = profile
			}

			// Self-healing: Name or Profile missing, try to fetch it
			if data.Name == "" || data.ProfileURL == "" {
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
						}
						if athlete.Profile != "" {
							data.ProfileURL = athlete.Profile
							session.Values["athlete_profile"] = athlete.Profile
						}
						session.Save(r, w)
					} else {
						log.Printf("Failed to auto-recover athlete data: %v", err)
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
