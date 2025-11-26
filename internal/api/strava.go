package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch activities, status: %s", resp.Status)
	}

	var activities []Activity
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		return nil, fmt.Errorf("failed to decode activities response: %w", err)
	}
	
	return activities, nil
}

