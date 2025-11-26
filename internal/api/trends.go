package api

import (
	"fmt"
	"math"
	"time"
)

// formatPaceSeconds formats pace in seconds to "X:XX" format (minutes:seconds).
func formatPaceSeconds(paceSec float64) string {
	totalSeconds := int(math.Round(paceSec))
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// TrendDataPoint represents a single data point in a trend chart.
type TrendDataPoint struct {
	Date          string  `json:"date"`           // YYYY-MM-DD format
	Distance      float64 `json:"distance"`     // in meters
	DistanceMiles float64 `json:"distance_miles"`
	Pace          string  `json:"pace"`          // formatted as "X:XX min/mi"
	PaceMinPerKm  string  `json:"pace_min_per_km"` // formatted as "X:XX min/km"
	Count         int     `json:"count"`         // number of activities
}

// TrendData represents aggregated trend data for a time period.
type TrendData struct {
	Period string           `json:"period"` // "daily", "weekly", "monthly"
	Points []TrendDataPoint  `json:"points"`
}

// CalculateTrends calculates trend data for activities aggregated by time period.
// period can be "daily", "weekly", or "monthly"
// runningOnly filters to only running activities if true
func CalculateTrends(activities []NormalizedActivity, period string, runningOnly bool) TrendData {
	// Filter to running activities if requested
	var filteredActivities []NormalizedActivity
	if runningOnly {
		for _, activity := range activities {
			// Use the same running activity check as in running.go
			if activity.SportType == "Run" || activity.SportType == "VirtualRun" || activity.SportType == "TrailRun" {
				filteredActivities = append(filteredActivities, activity)
			}
		}
	} else {
		filteredActivities = activities
	}

	if len(filteredActivities) == 0 {
		return TrendData{
			Period: period,
			Points: []TrendDataPoint{},
		}
	}

	// Group activities by time period
	grouped := groupByPeriod(filteredActivities, period)

	// Calculate averages for each period
	points := make([]TrendDataPoint, 0, len(grouped))
	for dateStr, group := range grouped {
		point := calculatePeriodAverage(group, dateStr)
		points = append(points, point)
	}

	// Sort by date
	sortTrendPoints(points)

	// Apply smoothing for daily data
	if period == "daily" && len(points) > 1 {
		points = applyMovingAverage(points, 3) // 3-day moving average
	}

	return TrendData{
		Period: period,
		Points: points,
	}
}

// groupByPeriod groups activities by the specified time period.
func groupByPeriod(activities []NormalizedActivity, period string) map[string][]NormalizedActivity {
	groups := make(map[string][]NormalizedActivity)

	for _, activity := range activities {
		dateKey := getPeriodKey(activity.LocalDateStr, period)
		groups[dateKey] = append(groups[dateKey], activity)
	}

	return groups
}

// getPeriodKey returns a date key for the specified period.
func getPeriodKey(dateStr string, period string) string {
	// Parse the date string (YYYY-MM-DD)
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr // Return as-is if parsing fails
	}

	switch period {
	case "weekly":
		// Get the start of the week (Monday)
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}
		weekStart := t.AddDate(0, 0, -(weekday - 1))
		return weekStart.Format("2006-01-02")
	case "monthly":
		// Get the first day of the month
		monthStart := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		return monthStart.Format("2006-01-02")
	default: // "daily"
		return dateStr
	}
}

// calculatePeriodAverage calculates the average distance and pace for a group of activities.
func calculatePeriodAverage(activities []NormalizedActivity, dateStr string) TrendDataPoint {
	var totalDistance float64
	var totalMovingTime int
	count := len(activities)

	for _, activity := range activities {
		totalDistance += activity.Distance
		totalMovingTime += activity.MovingTime
	}

	point := TrendDataPoint{
		Date:          dateStr,
		Distance:      totalDistance,
		DistanceMiles: totalDistance / 1609.34,
		Count:        count,
	}

	// Calculate average pace if we have distance and time
	if count > 0 && totalDistance > 0 && totalMovingTime > 0 {
		// Pace in seconds per meter
		paceSecPerMeter := float64(totalMovingTime) / totalDistance
		
		// Convert to min/mi
		paceSecPerMile := paceSecPerMeter * 1609.34
		point.Pace = formatPaceSeconds(paceSecPerMile)
		
		// Convert to min/km
		paceSecPerKm := paceSecPerMeter * 1000
		point.PaceMinPerKm = formatPaceSeconds(paceSecPerKm)
	}

	return point
}

// sortTrendPoints sorts trend points by date.
func sortTrendPoints(points []TrendDataPoint) {
	// Simple bubble sort (fine for small datasets)
	for i := 0; i < len(points)-1; i++ {
		for j := i + 1; j < len(points); j++ {
			if points[i].Date > points[j].Date {
				points[i], points[j] = points[j], points[i]
			}
		}
	}
}

// applyMovingAverage applies a moving average smoothing to trend points.
func applyMovingAverage(points []TrendDataPoint, window int) []TrendDataPoint {
	if len(points) <= window {
		return points
	}

	smoothed := make([]TrendDataPoint, len(points))
	halfWindow := window / 2

	for i := range points {
		smoothed[i] = points[i] // Copy the point

		// Calculate moving average for distance
		var sumDistance float64
		var count int

		start := i - halfWindow
		if start < 0 {
			start = 0
		}
		end := i + halfWindow + 1
		if end > len(points) {
			end = len(points)
		}

		for j := start; j < end; j++ {
			sumDistance += points[j].Distance
			if points[j].Distance > 0 {
				count++
			}
		}

		if count > 0 {
			avgDistance := sumDistance / float64(count)
			smoothed[i].Distance = avgDistance
			smoothed[i].DistanceMiles = avgDistance / 1609.34
		}

		// Note: Pace smoothing would require storing time separately
		// For now, pace remains as calculated from original data
	}

	return smoothed
}

