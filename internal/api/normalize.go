package api

import (
	"fmt"
	"time"
)

// NormalizedActivity represents an activity with normalized and computed fields.
type NormalizedActivity struct {
	Activity
	LocalDate      time.Time // Date portion of start_date_local (timezone-independent)
	LocalDateStr   string    // YYYY-MM-DD format for easy grouping
	DistanceKm     float64   // Distance in kilometers
	DistanceMiles  float64   // Distance in miles
	MovingTimeHours float64  // Moving time in hours
	MovingTimeFormatted string // Moving time formatted as "Xh Ym"
	ElevationGainMeters float64 // Elevation gain in meters
	ElevationGainFeet   float64 // Elevation gain in feet
	AverageSpeedKmh    float64  // Average speed in km/h
	AverageSpeedMph    float64  // Average speed in mph
	MaxSpeedKmh        float64  // Max speed in km/h
	MaxSpeedMph        float64  // Max speed in mph
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
		// Extract local date from start_date_local (timezone-independent)
		// The date components (year, month, day) are extracted regardless of timezone
		localDate := truncateToDate(activity.StartDateLocal)
		
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

// truncateToDate truncates a time to just the date portion (midnight in UTC).
// This extracts the date components (year, month, day) from the local time,
// ensuring activities are grouped by their local date regardless of timezone.
func truncateToDate(t time.Time) time.Time {
	// Extract date components from the time (this works regardless of timezone)
	year, month, day := t.Date()
	// Create a new time at midnight UTC with the same date components
	// This allows consistent date comparison regardless of the original timezone
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
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

