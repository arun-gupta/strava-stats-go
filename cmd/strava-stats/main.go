package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/arungupta/strava-stats-go/internal/api"
	"github.com/arungupta/strava-stats-go/internal/auth"
	"github.com/arungupta/strava-stats-go/internal/config"
	"golang.org/x/oauth2"
)

// ActivityCache stores fetched activities with a TTL
type ActivityCache struct {
	mu          sync.RWMutex
	cache       map[string]*CachedActivities
	ttl         time.Duration
}

type CachedActivities struct {
	Activities []api.Activity
	FetchedAt  time.Time
}

func NewActivityCache(ttl time.Duration) *ActivityCache {
	return &ActivityCache{
		cache: make(map[string]*CachedActivities),
		ttl:   ttl,
	}
}

// Get retrieves activities from cache if they exist and are not expired
func (c *ActivityCache) Get(key string) ([]api.Activity, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	cached, exists := c.cache[key]
	if !exists {
		return nil, false
	}
	
	// Check if expired
	if time.Since(cached.FetchedAt) > c.ttl {
		return nil, false
	}
	
	return cached.Activities, true
}

// Set stores activities in cache
func (c *ActivityCache) Set(key string, activities []api.Activity) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.cache[key] = &CachedActivities{
		Activities: activities,
		FetchedAt:  time.Now(),
	}
}

// Clear removes expired entries (called periodically)
func (c *ActivityCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	for key, cached := range c.cache {
		if now.Sub(cached.FetchedAt) > c.ttl {
			delete(c.cache, key)
		}
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize OAuth authenticator
	authenticator := auth.NewAuthenticator(cfg)

	// Initialize Strava API client
	stravaClient := api.NewClient(authenticator.StravaAPIURL, authenticator.Config)

	// Initialize activity cache (5 second TTL - enough for concurrent requests)
	activityCache := NewActivityCache(5 * time.Second)
	
	// Helper function to get or fetch activities with caching
	getOrFetchActivities := func(ctx context.Context, token *oauth2.Token, fetchOpts *api.FetchActivitiesOptions, cacheKey string) ([]api.Activity, error) {
		// Try cache first
		if cached, found := activityCache.Get(cacheKey); found {
			log.Printf("Using cached activities for key: %s (%d activities)", cacheKey, len(cached))
			return cached, nil
		}
		
		// Cache miss - fetch from Strava
		log.Printf("Cache miss for key: %s, fetching from Strava", cacheKey)
		activities, err := stravaClient.FetchAllActivities(ctx, token, fetchOpts)
		if err != nil {
			return nil, err
		}
		
		// Store in cache
		activityCache.Set(cacheKey, activities)
		log.Printf("Cached activities for key: %s (%d activities)", cacheKey, len(activities))
		
		return activities, nil
	}

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

		// Parse date range from query parameters FIRST, so we can use it for fetching
		var normalizeOpts *api.NormalizeOptions
		var fetchOpts *api.FetchActivitiesOptions
		startDateStr := r.URL.Query().Get("start_date")
		endDateStr := r.URL.Query().Get("end_date")
		
		if startDateStr != "" && endDateStr != "" {
			// Parse custom date range - parse as UTC to avoid timezone issues
			// The date string is YYYY-MM-DD format, which we want to treat as a date (not datetime)
			startDate, err1 := time.Parse("2006-01-02", startDateStr)
			endDate, err2 := time.Parse("2006-01-02", endDateStr)
			if err1 == nil && err2 == nil {
				// Ensure dates are in UTC and at midnight to avoid timezone comparison issues
				startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
				endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)
				normalizeOpts = &api.NormalizeOptions{
					StartDate: startDate,
					EndDate:   endDate,
				}
				
				// Use Strava's After parameter to only fetch activities after the start date
				// This is much more efficient than fetching all activities
				afterTimestamp := startDate.AddDate(0, 0, -1).Unix() // Fetch from 1 day before to be safe
				fetchOpts = &api.FetchActivitiesOptions{
					After: &afterTimestamp,
				}
				
				log.Printf("Using custom date range: %s to %s (parsed as %s to %s)", 
					startDateStr, endDateStr, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
			} else {
				log.Printf("Invalid date range parameters (start: %v, end: %v), using default (7 days)", err1, err2)
				normalizeOpts = nil
				fetchOpts = nil
			}
		} else {
			// Use default (7 days)
			log.Printf("No date range parameters provided, using default (7 days)")
			normalizeOpts = nil
			fetchOpts = nil
		}
		
		// Generate cache key from date range
		cacheKey := fmt.Sprintf("%s-%s", startDateStr, endDateStr)
		if cacheKey == "-" {
			cacheKey = "default"
		}
		
		// Fetch activities using cache helper
		activities, err := getOrFetchActivities(r.Context(), token, fetchOpts, cacheKey)
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
					activities, err = getOrFetchActivities(r.Context(), newToken, fetchOpts, cacheKey)
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
		
		// Normalize activities with date range
		normalized := api.NormalizeActivities(activities, normalizeOpts)

		// Debug: Log all normalized activity dates to see what we have
		log.Printf("Normalized activities count: %d", len(normalized))
		dateCounts := make(map[string]int)
		for _, activity := range normalized {
			if activity.LocalDateStr != "" {
				dateCounts[activity.LocalDateStr]++
			}
		}
		log.Printf("Activity dates: %v", dateCounts)
		
		// Debug: Log activities on Oct 28 (check both years)
		for _, activity := range normalized {
			if activity.LocalDateStr == "2024-10-28" || activity.LocalDateStr == "2025-10-28" {
				log.Printf("Found Oct 28 activity: ID=%d, Name=%s, LocalDateStr=%s, MovingTime=%d, StartDateLocal=%s", 
					activity.ID, activity.Name, activity.LocalDateStr, activity.MovingTime, activity.StartDateLocal)
			}
		}
		
		// Debug: Log activities on Nov 26 (check both years)
		for _, activity := range normalized {
			if activity.LocalDateStr == "2024-11-26" || activity.LocalDateStr == "2025-11-26" {
				log.Printf("Found Nov 26 activity: ID=%d, Name=%s, LocalDateStr=%s, MovingTime=%d, StartDateLocal=%s", 
					activity.ID, activity.Name, activity.LocalDateStr, activity.MovingTime, activity.StartDateLocal)
			}
		}
		
		// Debug: Check raw activities around Nov 26
		for _, activity := range activities {
			if !activity.StartDateLocal.IsZero() {
				dateStr := activity.StartDateLocal.Format("2006-01-02")
				if strings.Contains(dateStr, "-11-25") || strings.Contains(dateStr, "-11-26") || strings.Contains(dateStr, "-11-27") {
					log.Printf("Raw activity near Nov 26: ID=%d, Name=%s, StartDateLocal=%s, Timezone=%s, DateStr=%s", 
						activity.ID, activity.Name, activity.StartDateLocal.Format(time.RFC3339), activity.Timezone, dateStr)
				}
			}
		}
		
		// Debug: Check normalized activities around Nov 26
		for _, activity := range normalized {
			if !activity.StartDateLocal.IsZero() {
				originalDateStr := activity.StartDateLocal.Format("2006-01-02")
				if strings.Contains(originalDateStr, "-11-25") || strings.Contains(originalDateStr, "-11-26") || strings.Contains(originalDateStr, "-11-27") {
					log.Printf("Normalized activity near Nov 26: ID=%d, Name=%s, OriginalDate=%s, LocalDateStr=%s, Timezone=%s", 
						activity.ID, activity.Name, originalDateStr, activity.LocalDateStr, activity.Timezone)
				}
			}
		}
		
		// Debug: Check raw activities around Oct 28 (27, 28, 29) to see if timezone conversion is shifting dates
		for _, activity := range activities {
			if !activity.StartDateLocal.IsZero() {
				// Format as string to check dates around Oct 28
				dateStr := activity.StartDateLocal.Format("2006-01-02")
				if strings.Contains(dateStr, "-10-27") || strings.Contains(dateStr, "-10-28") || strings.Contains(dateStr, "-10-29") {
					log.Printf("Raw activity near Oct 28: ID=%d, Name=%s, StartDateLocal=%s, Timezone=%s, DateStr=%s", 
						activity.ID, activity.Name, activity.StartDateLocal.Format(time.RFC3339), activity.Timezone, dateStr)
				}
			}
		}
		
		// Debug: After normalization, check what dates Oct 27-29 activities ended up with
		for _, activity := range normalized {
			// Check if the original date was around Oct 28
			if !activity.StartDateLocal.IsZero() {
				originalDateStr := activity.StartDateLocal.Format("2006-01-02")
				if strings.Contains(originalDateStr, "-10-27") || strings.Contains(originalDateStr, "-10-28") || strings.Contains(originalDateStr, "-10-29") {
					log.Printf("Normalized activity near Oct 28: ID=%d, Name=%s, OriginalDate=%s, LocalDateStr=%s, Timezone=%s", 
						activity.ID, activity.Name, originalDateStr, activity.LocalDateStr, activity.Timezone)
				}
			}
		}

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
		
		// Debug: Log date range info
		if normalizeOpts != nil && !normalizeOpts.StartDate.IsZero() {
			log.Printf("Date range: %s to %s, normalized activities: %d, earliest: %s, latest: %s",
				normalizeOpts.StartDate.Format("2006-01-02"), normalizeOpts.EndDate.Format("2006-01-02"),
				len(normalized), earliestDateStr, latestDateStr)
		}

		// Format date range for display
		// Use the requested date range if available, otherwise use the actual activity date range
		var dateRange string
		var responseStartDateStr, responseEndDateStr string
		
		if normalizeOpts != nil && !normalizeOpts.StartDate.IsZero() && !normalizeOpts.EndDate.IsZero() {
			// Use the requested date range for display
			responseStartDateStr = normalizeOpts.StartDate.Format("2006-01-02")
			responseEndDateStr = normalizeOpts.EndDate.Format("2006-01-02")
			dateRange = fmt.Sprintf("%s - %s", 
				normalizeOpts.StartDate.Format("Jan 2"), 
				normalizeOpts.EndDate.Format("Jan 2"))
		} else if earliestDateStr != "" && latestDateStr != "" {
			// Fallback to actual activity date range if no explicit range was requested
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
			responseStartDateStr = earliestDateStr
			responseEndDateStr = latestDateStr
		} else {
			dateRange = "No activities"
		}

		// Format total moving time
		movingTimeFormatted := api.FormatDuration(totalMovingTime)

		// Prepare response
		response := map[string]interface{}{
			"dateRange":      dateRange,
			"startDate":      responseStartDateStr,
			"endDate":        responseEndDateStr,
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

		// Parse date range from query parameters FIRST, so we can use it for fetching
		var normalizeOpts *api.NormalizeOptions
		var fetchOpts *api.FetchActivitiesOptions
		startDateStr := r.URL.Query().Get("start_date")
		endDateStr := r.URL.Query().Get("end_date")
		
		if startDateStr != "" && endDateStr != "" {
			// Parse custom date range
			startDate, err1 := time.Parse("2006-01-02", startDateStr)
			endDate, err2 := time.Parse("2006-01-02", endDateStr)
			if err1 == nil && err2 == nil {
				// Ensure dates are in UTC and at midnight to avoid timezone comparison issues
				startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
				endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)
				normalizeOpts = &api.NormalizeOptions{
					StartDate: startDate,
					EndDate:   endDate,
				}
				
				// Use Strava's After parameter to only fetch activities after the start date
				afterTimestamp := startDate.AddDate(0, 0, -1).Unix() // Fetch from 1 day before to be safe
				fetchOpts = &api.FetchActivitiesOptions{
					After: &afterTimestamp,
				}
				
				log.Printf("Running stats: using custom date range: %s to %s", startDateStr, endDateStr)
			} else {
				log.Printf("Running stats: invalid date range parameters, using default (7 days)")
				normalizeOpts = nil
				fetchOpts = nil
			}
		} else {
			// Use default (7 days)
			normalizeOpts = nil
			fetchOpts = nil
		}
		
		// Generate cache key from date range
		cacheKey := fmt.Sprintf("%s-%s", startDateStr, endDateStr)
		if cacheKey == "-" {
			cacheKey = "default"
		}
		
		// Fetch activities using cache helper
		activities, err := getOrFetchActivities(r.Context(), token, fetchOpts, cacheKey)
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
							activities, err = getOrFetchActivities(r.Context(), newToken, fetchOpts, cacheKey)
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

		// Normalize activities with date range
		normalized := api.NormalizeActivities(activities, normalizeOpts)
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

	// API endpoint for trends data
	http.HandleFunc("/api/trends", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get query parameters
		period := r.URL.Query().Get("period")
		if period == "" {
			period = "daily" // default to daily
		}
		if period != "daily" && period != "weekly" && period != "monthly" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid period. Must be 'daily', 'weekly', or 'monthly'"})
			return
		}

		runningOnly := r.URL.Query().Get("running_only") == "true"

		// Get token from session
		token, err := authenticator.GetToken(w, r)
		if err != nil {
			log.Printf("Trends: unauthorized: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized: " + err.Error()})
			return
		}

		log.Printf("Trends: fetching activities for period=%s, runningOnly=%v", period, runningOnly)

		// Parse date range from query parameters FIRST, so we can use it for fetching
		var normalizeOpts *api.NormalizeOptions
		var fetchOpts *api.FetchActivitiesOptions
		trendsStartDateStr := r.URL.Query().Get("start_date")
		trendsEndDateStr := r.URL.Query().Get("end_date")
		
		if trendsStartDateStr != "" && trendsEndDateStr != "" {
			// Parse custom date range
			startDate, err1 := time.Parse("2006-01-02", trendsStartDateStr)
			endDate, err2 := time.Parse("2006-01-02", trendsEndDateStr)
			if err1 == nil && err2 == nil {
				// Ensure dates are in UTC and at midnight to avoid timezone comparison issues
				startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
				endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)
				normalizeOpts = &api.NormalizeOptions{
					StartDate: startDate,
					EndDate:   endDate,
				}
				
				// Use Strava's After parameter to only fetch activities after the start date
				afterTimestamp := startDate.AddDate(0, 0, -1).Unix() // Fetch from 1 day before to be safe
				fetchOpts = &api.FetchActivitiesOptions{
					After: &afterTimestamp,
				}
				
				log.Printf("Trends: using custom date range: %s to %s", trendsStartDateStr, trendsEndDateStr)
			} else {
				log.Printf("Trends: invalid date range parameters, using default (7 days)")
				normalizeOpts = nil
				fetchOpts = nil
			}
		} else {
			// Use default (7 days)
			normalizeOpts = nil
			fetchOpts = nil
		}
		
		// Generate cache key from date range
		cacheKey := fmt.Sprintf("%s-%s", trendsStartDateStr, trendsEndDateStr)
		if cacheKey == "-" {
			cacheKey = "default"
		}
		
		// Fetch activities using cache helper
		activities, err := getOrFetchActivities(r.Context(), token, fetchOpts, cacheKey)
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
					newToken, getErr := authenticator.GetToken(w, r)
					if getErr == nil {
						activities, err = getOrFetchActivities(r.Context(), newToken, fetchOpts, cacheKey)
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
				log.Printf("Trends: failed to fetch activities: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch activities: " + err.Error()})
				return
			}
		}

		log.Printf("Trends: fetched %d activities", len(activities))

		// Normalize activities with date range
		normalized := api.NormalizeActivities(activities, normalizeOpts)
		log.Printf("Trends: normalized to %d activities", len(normalized))

		// Calculate trends
		trendData := api.CalculateTrends(normalized, period, runningOnly)

		// Prepare response
		response := map[string]interface{}{
			"period":      period,
			"runningOnly": runningOnly,
			"trends":      trendData,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Trends: failed to encode response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to encode response: " + err.Error()})
			return
		}

		log.Printf("Trends: successfully returned %d data points for period=%s", len(trendData.Points), period)
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
