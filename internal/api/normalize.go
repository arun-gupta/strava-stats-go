package api

import (
	"fmt"
	"time"
)

// NormalizedActivity represents an activity with normalized and computed fields.
type NormalizedActivity struct {
	Activity
	LocalDate           time.Time `json:"local_date"`            // Date portion of start_date_local (timezone-independent)
	LocalDateStr        string    `json:"local_date_str"`        // YYYY-MM-DD format for easy grouping
	DistanceKm          float64   `json:"distance_km"`           // Distance in kilometers
	DistanceMiles       float64   `json:"distance_miles"`        // Distance in miles
	MovingTimeHours     float64   `json:"moving_time_hours"`     // Moving time in hours
	MovingTimeFormatted string    `json:"moving_time_formatted"`  // Moving time formatted as "Xh Ym"
	ElevationGainMeters float64   `json:"elevation_gain_meters"` // Elevation gain in meters
	ElevationGainFeet   float64   `json:"elevation_gain_feet"`   // Elevation gain in feet
	AverageSpeedKmh     float64   `json:"average_speed_kmh"`      // Average speed in km/h
	AverageSpeedMph      float64   `json:"average_speed_mph"`      // Average speed in mph
	MaxSpeedKmh          float64   `json:"max_speed_kmh"`        // Max speed in km/h
	MaxSpeedMph          float64   `json:"max_speed_mph"`        // Max speed in mph
}

// NormalizeOptions contains options for normalizing activities.
type NormalizeOptions struct {
	DaysBack  int       // Number of days to look back (default: 7) - used if StartDate/EndDate not set
	StartDate time.Time // Start date for filtering (inclusive) - if set, overrides DaysBack
	EndDate   time.Time // End date for filtering (inclusive) - if set, overrides DaysBack
}

// NormalizeActivities normalizes a slice of activities:
// - Filters to date range based on local date (timezone-independent)
// - Extracts local date for grouping
// - Standardizes units (metric base with imperial conversions)
func NormalizeActivities(activities []Activity, opts *NormalizeOptions) []NormalizedActivity {
	if opts == nil {
		opts = &NormalizeOptions{DaysBack: 7}
	}

	// Determine date range
	var startDate, endDate time.Time
	now := truncateToDate(time.Now())
	
	if !opts.StartDate.IsZero() && !opts.EndDate.IsZero() {
		// Use explicit date range
		startDate = truncateToDate(opts.StartDate)
		endDate = truncateToDate(opts.EndDate)
	} else {
		// Use DaysBack (default behavior)
		if opts.DaysBack <= 0 {
			opts.DaysBack = 7
		}
		startDate = now.AddDate(0, 0, -opts.DaysBack)
		endDate = now
	}

		var normalized []NormalizedActivity
		for _, activity := range activities {
			// Extract local date from start_date_local
			// Use the timezone from the activity if available to ensure correct date extraction
			localDate := extractLocalDate(activity.StartDateLocal, activity.Timezone)
			localDateTruncated := truncateToDate(localDate)
			localDateStr := localDateTruncated.Format("2006-01-02")
			
			// Debug: Log activities around Oct 28 and Nov 26 to see what's happening
			if localDateStr == "2025-10-27" || localDateStr == "2025-10-28" || localDateStr == "2025-10-29" ||
			   localDateStr == "2025-11-25" || localDateStr == "2025-11-26" || localDateStr == "2025-11-27" {
				fmt.Printf("DEBUG normalize: ID=%d, Name=%s, StartDateLocal=%s, Timezone=%s, LocalDateStr=%s, StartDate=%s, EndDate=%s, Before=%v, After=%v\n",
					activity.ID, activity.Name, activity.StartDateLocal.Format(time.RFC3339), activity.Timezone,
					localDateStr, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"),
					localDateTruncated.Before(startDate), localDateTruncated.After(endDate))
			}
			
			// Filter: only include activities within the date range (inclusive on both ends)
			// This ensures activities are included based on their local date, not UTC
			// Use !Before and !After to make it inclusive
			if localDateTruncated.Before(startDate) || localDateTruncated.After(endDate) {
				if localDateStr == "2025-10-27" || localDateStr == "2025-10-28" || localDateStr == "2025-10-29" {
					fmt.Printf("DEBUG: Activity filtered out: ID=%d, LocalDateStr=%s, StartDate=%s, EndDate=%s\n",
						activity.ID, localDateStr, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
				}
				continue
			}

			normalized = append(normalized, normalizeActivity(activity, localDate))
		}

	return normalized
}

// normalizeActivity normalizes a single activity.
func normalizeActivity(activity Activity, localDate time.Time) NormalizedActivity {
	norm := NormalizedActivity{
		Activity:  activity,
		LocalDate: localDate,
		LocalDateStr: localDate.Format("2006-01-02"),
	}

	// Distance conversions (meters to km and miles)
	norm.DistanceKm = activity.Distance / 1000.0
	norm.DistanceMiles = activity.Distance / 1609.34

	// Time conversions (seconds to hours and formatted string)
	norm.MovingTimeHours = float64(activity.MovingTime) / 3600.0
	norm.MovingTimeFormatted = FormatDuration(activity.MovingTime)

	// Elevation conversions (meters to feet)
	norm.ElevationGainMeters = activity.TotalElevationGain
	norm.ElevationGainFeet = activity.TotalElevationGain * 3.28084

	// Speed conversions (m/s to km/h and mph)
	norm.AverageSpeedKmh = activity.AverageSpeed * 3.6 // m/s to km/h
	norm.AverageSpeedMph = activity.AverageSpeed * 2.23694 // m/s to mph
	norm.MaxSpeedKmh = activity.MaxSpeed * 3.6
	norm.MaxSpeedMph = activity.MaxSpeed * 2.23694

	return norm
}

// extractLocalDate extracts the local date from a time, using the timezone if provided.
// The key insight: start_date_local from Strava is the local time, but when Go's JSON
// unmarshaler parses it, it might interpret it as UTC. However, the date components
// (year, month, day) in the original string are the local date we want.
// 
// The solution: Extract date components from the time as if it represents local time.
// If the timezone is provided, we can use it to properly interpret the time.
// Otherwise, we extract the date components directly, which works because the
// date part of start_date_local is what we want regardless of timezone conversion.
func extractLocalDate(t time.Time, timezone string) time.Time {
	// CRITICAL: The user wants activities to be counted based on the day they occurred
	// in the local timezone, independent of UTC conversion.
	//
	// PROBLEM: Strava's `start_date_local` field is named "local" but when Go's JSON
	// unmarshaler parses a string like "2025-11-26T06:04:47Z", the Z means UTC, so it
	// converts it to UTC time. However, the date part (2025-11-26) in the original string
	// represents the LOCAL date we want.
	//
	// OBSERVATION: From the logs, we see that activities like "Fartleks" have:
	// - StartDateLocal=2025-11-26T06:04:47Z (6:04 AM UTC on Nov 26)
	// - When converted to Pacific Time: 2025-11-25T22:04:47-08:00 (10:04 PM on Nov 25)
	// - But the user wants it to be Nov 26
	//
	// SOLUTION: Extract the date from the UTC time directly, since Strava's `start_date_local`
	// appears to send the local date as if it were UTC. The date component (YYYY-MM-DD) in
	// the UTC time represents the local date we want.
	
	// Extract date from UTC time - the date part represents the local date
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// truncateToDate truncates a time to just the date portion (midnight in UTC).
// This extracts the date components (year, month, day) from the local time,
// ensuring activities are grouped by their local date regardless of timezone.
func truncateToDate(t time.Time) time.Time {
	// Format the date as YYYY-MM-DD using the timezone of the time.Time value.
	// This preserves the local date representation before extracting components.
	dateStr := t.Format("2006-01-02")
	
	// Parse the date string to extract year, month, day
	// This ensures we get the date as it appears in the original timezone
	parsed, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		// Fallback to Date() method if parsing fails
		year, month, day := t.Date()
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}
	
	// Return as UTC midnight with the same date components
	// This allows consistent date comparison regardless of the original timezone
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
}

// FormatDuration formats seconds into a human-readable string like "2h 30m" or "45m".
func FormatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		if minutes > 0 {
			if secs > 0 {
				return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
			}
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		if secs > 0 {
			return fmt.Sprintf("%dh %ds", hours, secs)
		}
		return fmt.Sprintf("%dh", hours)
	}

	if secs > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%dm", minutes)
}

