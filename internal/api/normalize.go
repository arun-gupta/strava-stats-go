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
	DaysBack int // Number of days to look back (default: 7)
}

// NormalizeActivities normalizes a slice of activities:
// - Filters to last N days based on local date (timezone-independent)
// - Extracts local date for grouping
// - Standardizes units (metric base with imperial conversions)
func NormalizeActivities(activities []Activity, opts *NormalizeOptions) []NormalizedActivity {
	if opts == nil {
		opts = &NormalizeOptions{DaysBack: 7}
	}
	if opts.DaysBack <= 0 {
		opts.DaysBack = 7
	}

	// Calculate cutoff date (today in local time, minus days back)
	now := time.Now()
	cutoffDate := truncateToDate(now).AddDate(0, 0, -opts.DaysBack)

	var normalized []NormalizedActivity
	for _, activity := range activities {
		// Extract local date from start_date_local
		// Use the timezone from the activity if available to ensure correct date extraction
		localDate := extractLocalDate(activity.StartDateLocal, activity.Timezone)
		
		// Filter: only include activities that occurred on or after the cutoff date
		// This ensures activities are included based on their local date, not UTC
		// Activities are included if they occurred on the cutoff date or later
		if localDate.Before(cutoffDate) {
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
	// The key insight: start_date_local from Strava represents local time in the user's timezone.
	// When Go's JSON unmarshaler parses it:
	// - If the string has a timezone offset (e.g., "2024-11-25T23:00:00-08:00"), it converts to UTC
	// - If the string has no timezone, it might parse as UTC or local time
	//
	// Problem: If it's converted to UTC, the UTC date might be different from the local date.
	// Example: "2024-11-25T23:00:00-08:00" (11 PM PST) = "2024-11-26T07:00:00Z" (7 AM UTC next day)
	// - UTC date: Nov 26
	// - Local date: Nov 25 (what we want)
	//
	// Solution: Use the timezone field to convert back to local time, then extract the date.
	if timezone != "" {
		loc, err := time.LoadLocation(timezone)
		if err == nil {
			// Convert to local timezone and extract date
			localTime := t.In(loc)
			year, month, day := localTime.Date()
			return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		}
		// If LoadLocation fails, try common timezone name mappings
		// Strava might return formats like "(GMT-08:00) Pacific Time" or "America/Los_Angeles"
	}
	
	// Fallback: Extract date from the time as-is.
	// This works if the time wasn't converted to UTC, or if the date components
	// happen to match the local date. Not perfect, but better than nothing.
	year, month, day := t.Date()
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

