package api

import (
	"testing"
	"time"
)

func TestTruncateToDate(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "UTC time",
			input:    time.Date(2024, 11, 25, 14, 30, 0, 0, time.UTC),
			expected: time.Date(2024, 11, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Local timezone (PST)",
			input:    time.Date(2024, 11, 25, 14, 30, 0, 0, time.FixedZone("PST", -8*3600)),
			expected: time.Date(2024, 11, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Midnight",
			input:    time.Date(2024, 11, 25, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 11, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Late evening (next day in UTC)",
			input:    time.Date(2024, 11, 25, 23, 59, 59, 0, time.FixedZone("EST", -5*3600)),
			expected: time.Date(2024, 11, 25, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToDate(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("truncateToDate(%v) = %v, want %v", tt.input, result, tt.expected)
			}
			// Verify it's just the date (time should be midnight)
			if result.Hour() != 0 || result.Minute() != 0 || result.Second() != 0 {
				t.Errorf("truncateToDate should return midnight, got %v", result)
			}
		})
	}
}

func TestNormalizeActivities_Last7Days(t *testing.T) {
	now := time.Now()
	
	// Create activities spanning different dates
	activities := []Activity{
		{
			ID:             1,
			Name:           "Today's run",
			StartDateLocal: now,
			Distance:       5000, // 5km
			MovingTime:     1800, // 30 minutes
		},
		{
			ID:             2,
			Name:           "3 days ago",
			StartDateLocal: now.AddDate(0, 0, -3),
			Distance:       10000, // 10km
			MovingTime:     3600,  // 1 hour
		},
		{
			ID:             3,
			Name:           "8 days ago (should be filtered)",
			StartDateLocal: now.AddDate(0, 0, -8),
			Distance:       3000,
			MovingTime:     900,
		},
		{
			ID:             4,
			Name:           "Exactly 7 days ago (should be included)",
			StartDateLocal: now.AddDate(0, 0, -7),
			Distance:       8000,
			MovingTime:     2700,
		},
	}

	normalized := NormalizeActivities(activities, nil)

	// Should include today, 3 days ago, and exactly 7 days ago (3 activities)
	// Should exclude 8 days ago
	if len(normalized) != 3 {
		t.Errorf("Expected 3 normalized activities, got %d", len(normalized))
	}

	// Verify all returned activities are within the last 7 days
	cutoff := truncateToDate(now).AddDate(0, 0, -7)
	for _, norm := range normalized {
		if norm.LocalDate.Before(cutoff) {
			t.Errorf("Activity %d has date %v which is before cutoff %v", norm.ID, norm.LocalDate, cutoff)
		}
	}
}

func TestNormalizeActivities_CustomDaysBack(t *testing.T) {
	now := time.Now()
	
	activities := []Activity{
		{
			ID:             1,
			StartDateLocal: now.AddDate(0, 0, -5),
			Distance:       5000,
			MovingTime:     1800,
		},
		{
			ID:             2,
			StartDateLocal: now.AddDate(0, 0, -10),
			Distance:       10000,
			MovingTime:     3600,
		},
	}

	opts := &NormalizeOptions{DaysBack: 10}
	normalized := NormalizeActivities(activities, opts)

	if len(normalized) != 2 {
		t.Errorf("Expected 2 normalized activities with 10 days back, got %d", len(normalized))
	}
}

func TestNormalizeActivity_UnitConversions(t *testing.T) {
	activity := Activity{
		ID:                1,
		Distance:          5000,        // 5km in meters
		MovingTime:        3600,        // 1 hour in seconds
		TotalElevationGain: 100,        // 100 meters
		AverageSpeed:      2.77778,    // ~10 km/h in m/s
		MaxSpeed:          5.55556,    // ~20 km/h in m/s
	}

	localDate := truncateToDate(time.Now())
	normalized := normalizeActivity(activity, localDate)

	// Distance conversions
	if normalized.DistanceKm != 5.0 {
		t.Errorf("Expected 5.0 km, got %f", normalized.DistanceKm)
	}
	expectedMiles := 5000.0 / 1609.34
	if normalized.DistanceMiles < expectedMiles-0.01 || normalized.DistanceMiles > expectedMiles+0.01 {
		t.Errorf("Expected ~%f miles, got %f", expectedMiles, normalized.DistanceMiles)
	}

	// Time conversions
	if normalized.MovingTimeHours != 1.0 {
		t.Errorf("Expected 1.0 hour, got %f", normalized.MovingTimeHours)
	}
	if normalized.MovingTimeFormatted != "1h" {
		t.Errorf("Expected '1h', got %s", normalized.MovingTimeFormatted)
	}

	// Elevation conversions
	if normalized.ElevationGainMeters != 100.0 {
		t.Errorf("Expected 100.0 meters, got %f", normalized.ElevationGainMeters)
	}
	expectedFeet := 100.0 * 3.28084
	if normalized.ElevationGainFeet < expectedFeet-0.1 || normalized.ElevationGainFeet > expectedFeet+0.1 {
		t.Errorf("Expected ~%f feet, got %f", expectedFeet, normalized.ElevationGainFeet)
	}

	// Speed conversions (approximately)
	expectedKmh := 2.77778 * 3.6 // ~10 km/h
	if normalized.AverageSpeedKmh < expectedKmh-0.1 || normalized.AverageSpeedKmh > expectedKmh+0.1 {
		t.Errorf("Expected ~%f km/h, got %f", expectedKmh, normalized.AverageSpeedKmh)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  int
		expected string
	}{
		{0, "0s"},
		{30, "30s"},
		{60, "1m"},
		{90, "1m 30s"},
		{3600, "1h"},
		{3660, "1h 1m"},
		{3720, "1h 2m"},
		{3780, "1h 3m"},
		{7200, "2h"},
		{7260, "2h 1m"},
		{7320, "2h 2m"},
		{7380, "2h 3m"},
		{36000, "10h"},
		{36600, "10h 10m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatDuration(tt.seconds)
			if result != tt.expected {
				t.Errorf("FormatDuration(%d) = %s, want %s", tt.seconds, result, tt.expected)
			}
		})
	}
}

func TestNormalizeActivities_TimezoneIndependent(t *testing.T) {
	// Test that activities are grouped by local date regardless of timezone
	// An activity at 11 PM PST on Nov 25 should be grouped with Nov 25,
	// even though it might be Nov 26 in UTC
	
	pst := time.FixedZone("PST", -8*3600)
	utc := time.UTC
	
	// Same calendar date, different timezones
	activity1 := Activity{
		ID:             1,
		StartDateLocal: time.Date(2024, 11, 25, 23, 0, 0, 0, pst), // Nov 25, 11 PM PST
		Distance:       5000,
		MovingTime:     1800,
	}
	
	activity2 := Activity{
		ID:             2,
		StartDateLocal: time.Date(2024, 11, 25, 7, 0, 0, 0, utc), // Nov 25, 7 AM UTC (Nov 24, 11 PM PST)
		Distance:       3000,
		MovingTime:     1200,
	}

	activities := []Activity{activity1, activity2}
	
	// Both should be included if they fall within the date range
	// The key is that they're grouped by their local date, not UTC date
	normalized := NormalizeActivities(activities, &NormalizeOptions{DaysBack: 7})
	
	// Verify local dates are extracted correctly
	for _, norm := range normalized {
		year, month, day := norm.StartDateLocal.Date()
		expectedDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		if !norm.LocalDate.Equal(expectedDate) {
			t.Errorf("LocalDate mismatch: got %v, expected %v (from %v)", 
				norm.LocalDate, expectedDate, norm.StartDateLocal)
		}
		if norm.LocalDateStr != expectedDate.Format("2006-01-02") {
			t.Errorf("LocalDateStr mismatch: got %s, expected %s", 
				norm.LocalDateStr, expectedDate.Format("2006-01-02"))
		}
	}
}

