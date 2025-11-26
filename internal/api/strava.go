package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
)

// Client handles Strava API requests.
type Client struct {
	APIURL      string
	OAuthConfig *oauth2.Config
}

// NewClient creates a new Strava API client.
func NewClient(apiURL string, oauthConfig *oauth2.Config) *Client {
	return &Client{
		APIURL:      apiURL,
		OAuthConfig: oauthConfig,
	}
}

// Activity represents a Strava activity.
type Activity struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	SportType         string    `json:"sport_type"`
	StartDate         time.Time `json:"start_date"`
	StartDateLocal    time.Time `json:"start_date_local"`
	Timezone          string    `json:"timezone"`
	MovingTime        int       `json:"moving_time"`        // in seconds
	ElapsedTime       int       `json:"elapsed_time"`       // in seconds
	Distance          float64   `json:"distance"`           // in meters
	TotalElevationGain float64  `json:"total_elevation_gain"` // in meters
	AverageSpeed      float64   `json:"average_speed"`       // in meters per second
	MaxSpeed          float64   `json:"max_speed"`          // in meters per second
	AverageCadence    float64   `json:"average_cadence"`
	AverageWatts       float64   `json:"average_watts"`
	WeightedAverageWatts float64 `json:"weighted_average_watts"`
	Kilojoules        float64   `json:"kilojoules"`
	HasHeartrate      bool      `json:"has_heartrate"`
	AverageHeartrate  float64   `json:"average_heartrate"`
	MaxHeartrate      float64   `json:"max_heartrate"`
	ElevHigh          float64   `json:"elev_high"`          // in meters
	ElevLow           float64   `json:"elev_low"`           // in meters
	WorkoutType       *int      `json:"workout_type"`
}

// FetchActivitiesOptions contains optional parameters for fetching activities.
type FetchActivitiesOptions struct {
	Before *int64 // Unix timestamp
	After  *int64 // Unix timestamp
	Page   *int   // Page number (default: 1)
	PerPage *int  // Number of items per page (default: 30, max: 200)
}

// APIError represents an error from the Strava API.
type APIError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration // For rate limit errors
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Strava API error (status %d): %s", e.StatusCode, e.Message)
}

// IsRateLimit returns true if this is a rate limit error (429).
func (e *APIError) IsRateLimit() bool {
	return e.StatusCode == http.StatusTooManyRequests
}

// IsUnauthorized returns true if this is an unauthorized error (401).
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// IsServerError returns true if this is a server error (5xx).
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// FetchActivities retrieves the authenticated athlete's activities from Strava API.
func (c *Client) FetchActivities(ctx context.Context, token *oauth2.Token, opts *FetchActivitiesOptions) ([]Activity, error) {
	client := c.OAuthConfig.Client(ctx, token)
	
	url := fmt.Sprintf("%s/athlete/activities", c.APIURL)
	
	// Build query parameters
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	q := req.URL.Query()
	if opts != nil {
		if opts.Before != nil {
			q.Set("before", fmt.Sprintf("%d", *opts.Before))
		}
		if opts.After != nil {
			q.Set("after", fmt.Sprintf("%d", *opts.After))
		}
		if opts.Page != nil {
			q.Set("page", fmt.Sprintf("%d", *opts.Page))
		}
		if opts.PerPage != nil {
			q.Set("per_page", fmt.Sprintf("%d", *opts.PerPage))
		}
	}
	req.URL.RawQuery = q.Encode()
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch activities: %w", err)
	}
	defer resp.Body.Close()

	// Handle different HTTP status codes
	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
		}

		// Read error response body if available
		body, readErr := io.ReadAll(resp.Body)
		if readErr == nil && len(body) > 0 {
			// Try to parse as JSON error response
			var errorResp struct {
				Message string `json:"message"`
				Errors  []struct {
					Resource string `json:"resource"`
					Field    string `json:"field"`
					Code     string `json:"code"`
				} `json:"errors"`
			}
			if json.Unmarshal(body, &errorResp) == nil && errorResp.Message != "" {
				apiErr.Message = errorResp.Message
			} else {
				apiErr.Message = string(body)
			}
		} else {
			apiErr.Message = resp.Status
		}

		// Handle rate limiting (429)
		if resp.StatusCode == http.StatusTooManyRequests {
			// Check for Retry-After header
			if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
				if seconds, err := strconv.Atoi(retryAfterStr); err == nil {
					apiErr.RetryAfter = time.Duration(seconds) * time.Second
				}
			}
			// Default retry after 60 seconds if not specified
			if apiErr.RetryAfter == 0 {
				apiErr.RetryAfter = 60 * time.Second
			}
			return nil, apiErr
		}

		// Handle unauthorized (401) - token may need refresh
		if resp.StatusCode == http.StatusUnauthorized {
			apiErr.Message = "Unauthorized: token may be expired or invalid"
			return nil, apiErr
		}

		// Handle server errors (5xx)
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			apiErr.Message = fmt.Sprintf("Strava API server error: %s", apiErr.Message)
			return nil, apiErr
		}

		// Other errors
		return nil, apiErr
	}

	var activities []Activity
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		return nil, fmt.Errorf("failed to decode activities response: %w", err)
	}
	
	return activities, nil
}

// FetchAllActivities retrieves all activities from Strava API by paginating through all pages.
// This function automatically handles pagination to fetch the complete activity history.
func (c *Client) FetchAllActivities(ctx context.Context, token *oauth2.Token, opts *FetchActivitiesOptions) ([]Activity, error) {
	var allActivities []Activity
	page := 1
	perPage := 200 // Use max per_page to minimize number of requests
	
	// If opts is provided, use its parameters but override page and per_page for pagination
	paginationOpts := &FetchActivitiesOptions{
		Before:  opts.Before,
		After:   opts.After,
		PerPage: &perPage,
	}
	
	for {
		// Set current page
		paginationOpts.Page = &page
		
		// Fetch current page
		activities, err := c.FetchActivities(ctx, token, paginationOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch activities page %d: %w", page, err)
		}
		
		// If no activities returned, we've reached the end
		if len(activities) == 0 {
			break
		}
		
		// Append activities from this page
		allActivities = append(allActivities, activities...)
		
		// If we got fewer activities than per_page, we've reached the last page
		if len(activities) < perPage {
			break
		}
		
		// Move to next page
		page++
	}
	
	return allActivities, nil
}

