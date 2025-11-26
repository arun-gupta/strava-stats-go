package api

import (
	"fmt"
	"math"
)

// RunningStats contains aggregated running statistics.
type RunningStats struct {
	TotalRuns      int     `json:"total_runs"`
	RunsOver10K    int     `json:"runs_over_10k"`
	TotalDistance  float64 `json:"total_distance"`  // in meters
	TotalDistanceMiles float64 `json:"total_distance_miles"`
	AveragePace    string  `json:"average_pace"`   // formatted as "X:XX min/mi"
	AveragePaceMinPerKm string `json:"average_pace_min_per_km"` // formatted as "X:XX min/km"
}

// PersonalRecords contains personal best records.
type PersonalRecords struct {
	FastestMile    *RunRecord `json:"fastest_mile,omitempty"`
	Fastest10K     *RunRecord `json:"fastest_10k,omitempty"`
	LongestRun     *RunRecord `json:"longest_run,omitempty"`
	MostElevation  *RunRecord `json:"most_elevation,omitempty"`
}

// RunRecord represents a single run with its metrics.
type RunRecord struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Date            string  `json:"date"`            // YYYY-MM-DD
	Distance        float64 `json:"distance"`      // in meters
	DistanceMiles   float64 `json:"distance_miles"`
	MovingTime      int     `json:"moving_time"`   // in seconds
	Pace            string  `json:"pace"`          // formatted as "X:XX min/mi"
	PaceMinPerKm    string  `json:"pace_min_per_km"` // formatted as "X:XX min/km"
	ElevationGain   float64 `json:"elevation_gain"` // in meters
	ElevationGainFeet float64 `json:"elevation_gain_feet"`
}

// DistanceHistogram represents a histogram of run distances.
type DistanceHistogram struct {
	Bins []HistogramBin `json:"bins"`
}

// HistogramBin represents a single bin in the histogram.
type HistogramBin struct {
	Range      string `json:"range"`      // e.g., "0-1 mi", "1-2 mi"
	RangeKm    string `json:"range_km"`  // e.g., "0-1 km", "1-2 km"
	Count      int    `json:"count"`
	Distance   float64 `json:"distance"`  // total distance in this bin (meters)
	DistanceMiles float64 `json:"distance_miles"`
}

// isRunningActivity checks if an activity is a running-related activity.
// Includes: Run, VirtualRun, TrailRun, etc.
func isRunningActivity(sportType string) bool {
	runningTypes := map[string]bool{
		"Run":        true,
		"VirtualRun": true,
		"TrailRun":   true,
	}
	return runningTypes[sportType]
}

// CalculateRunningStats calculates running statistics from normalized activities.
func CalculateRunningStats(activities []NormalizedActivity) RunningStats {
	var stats RunningStats
	var totalDistance float64
	var totalMovingTime int
	var runsOver10K int

	for _, activity := range activities {
		if !isRunningActivity(activity.SportType) {
			continue
		}

		stats.TotalRuns++
		totalDistance += activity.Distance
		totalMovingTime += activity.MovingTime

		// Count runs over 10K (10000 meters)
		if activity.Distance >= 10000 {
			runsOver10K++
		}
	}

	stats.RunsOver10K = runsOver10K
	stats.TotalDistance = totalDistance
	stats.TotalDistanceMiles = totalDistance / 1609.34

	// Calculate average pace (min/mi and min/km)
	if stats.TotalRuns > 0 && totalDistance > 0 {
		// Average pace = total time / total distance
		// Pace in seconds per meter
		paceSecPerMeter := float64(totalMovingTime) / totalDistance
		
		// Convert to min/mi
		paceSecPerMile := paceSecPerMeter * 1609.34
		stats.AveragePace = formatPace(paceSecPerMile)
		
		// Convert to min/km
		paceSecPerKm := paceSecPerMeter * 1000
		stats.AveragePaceMinPerKm = formatPace(paceSecPerKm)
	}

	return stats
}

// CalculatePersonalRecords finds personal records from running activities.
func CalculatePersonalRecords(activities []NormalizedActivity) PersonalRecords {
	var prs PersonalRecords
	const mileInMeters = 1609.34
	const tenKInMeters = 10000.0
	const mileTolerance = 200.0 // 200 meters tolerance for "mile" runs
	const tenKTolerance = 500.0 // 500 meters tolerance for "10K" runs

	var fastestMileTime int = -1
	var fastest10KTime int = -1
	var longestDistance float64 = -1
	var mostElevation float64 = -1

	for _, activity := range activities {
		if !isRunningActivity(activity.SportType) {
			continue
		}

		// Fastest mile: find runs between 0.9 and 1.1 miles
		if activity.Distance >= (mileInMeters-mileTolerance) && activity.Distance <= (mileInMeters+mileTolerance) {
			if fastestMileTime == -1 || activity.MovingTime < fastestMileTime {
				fastestMileTime = activity.MovingTime
				prs.FastestMile = createRunRecord(activity)
			}
		}

		// Fastest 10K: find runs between 9.5K and 10.5K
		if activity.Distance >= (tenKInMeters-tenKTolerance) && activity.Distance <= (tenKInMeters+tenKTolerance) {
			if fastest10KTime == -1 || activity.MovingTime < fastest10KTime {
				fastest10KTime = activity.MovingTime
				prs.Fastest10K = createRunRecord(activity)
			}
		}

		// Longest run
		if activity.Distance > longestDistance {
			longestDistance = activity.Distance
			prs.LongestRun = createRunRecord(activity)
		}

		// Most elevation gain
		if activity.TotalElevationGain > mostElevation {
			mostElevation = activity.TotalElevationGain
			prs.MostElevation = createRunRecord(activity)
		}
	}

	return prs
}

// GenerateDistanceHistogram creates a histogram of run distances.
// Uses 1-mile bins (or 1-km bins) for grouping.
func GenerateDistanceHistogram(activities []NormalizedActivity, useMiles bool) DistanceHistogram {
	var histogram DistanceHistogram
	histogram.Bins = []HistogramBin{}

	// Filter to only running activities
	var runs []NormalizedActivity
	for _, activity := range activities {
		if isRunningActivity(activity.SportType) {
			runs = append(runs, activity)
		}
	}

	if len(runs) == 0 {
		return histogram
	}

	// Determine bin size and max distance
	binSize := 1000.0 // 1 km in meters
	if useMiles {
		binSize = 1609.34 // 1 mile in meters
	}

	// Find max distance to determine number of bins
	maxDistance := 0.0
	for _, run := range runs {
		if run.Distance > maxDistance {
			maxDistance = run.Distance
		}
	}

	// Create bins (up to max distance, plus one more for overflow)
	numBins := int(math.Ceil(maxDistance/binSize)) + 1
	if numBins > 50 {
		numBins = 50 // Cap at 50 bins to avoid too many
	}

	// Initialize bins
	bins := make([]HistogramBin, numBins)
	for i := 0; i < numBins; i++ {
		binStart := float64(i) * binSize
		binEnd := float64(i+1) * binSize
		
		var rangeLabel, rangeLabelKm string
		if useMiles {
			rangeLabel = formatRangeMiles(binStart, binEnd)
			rangeLabelKm = formatRangeKm(binStart, binEnd)
		} else {
			rangeLabelKm = formatRangeKm(binStart, binEnd)
			rangeLabel = formatRangeMiles(binStart, binEnd)
		}

		bins[i] = HistogramBin{
			Range:   rangeLabel,
			RangeKm: rangeLabelKm,
			Count:   0,
			Distance: 0,
			DistanceMiles: 0,
		}
	}

	// Populate bins
	for _, run := range runs {
		binIndex := int(math.Floor(run.Distance / binSize))
		if binIndex >= numBins {
			binIndex = numBins - 1 // Put overflow in last bin
		}
		if binIndex >= 0 && binIndex < numBins {
			bins[binIndex].Count++
			bins[binIndex].Distance += run.Distance
			bins[binIndex].DistanceMiles += run.Distance / 1609.34
		}
	}

	// Filter out empty bins at the end
	for i := len(bins) - 1; i >= 0; i-- {
		if bins[i].Count > 0 {
			histogram.Bins = bins[:i+1]
			break
		}
	}

	return histogram
}

// createRunRecord creates a RunRecord from a NormalizedActivity.
func createRunRecord(activity NormalizedActivity) *RunRecord {
	record := &RunRecord{
		ID:              activity.ID,
		Name:            activity.Name,
		Date:            activity.LocalDateStr,
		Distance:        activity.Distance,
		DistanceMiles:   activity.DistanceMiles,
		MovingTime:      activity.MovingTime,
		ElevationGain:   activity.TotalElevationGain,
		ElevationGainFeet: activity.ElevationGainFeet,
	}

	// Calculate pace (min/mi and min/km)
	if activity.Distance > 0 && activity.MovingTime > 0 {
		// Pace in seconds per meter
		paceSecPerMeter := float64(activity.MovingTime) / activity.Distance
		
		// Convert to min/mi
		paceSecPerMile := paceSecPerMeter * 1609.34
		record.Pace = formatPace(paceSecPerMile)
		
		// Convert to min/km
		paceSecPerKm := paceSecPerMeter * 1000
		record.PaceMinPerKm = formatPace(paceSecPerKm)
	}

	return record
}

// formatPace formats pace in seconds to "X:XX min/mi" or "X:XX min/km" format.
func formatPace(paceSec float64) string {
	totalSeconds := int(math.Round(paceSec))
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// formatRangeMiles formats a distance range in miles.
func formatRangeMiles(startMeters, endMeters float64) string {
	startMi := startMeters / 1609.34
	endMi := endMeters / 1609.34
	
	// Round to 1 decimal place
	startMi = math.Round(startMi*10) / 10
	endMi = math.Round(endMi*10) / 10
	
	return fmt.Sprintf("%.1f-%.1f mi", startMi, endMi)
}

// formatRangeKm formats a distance range in kilometers.
func formatRangeKm(startMeters, endMeters float64) string {
	startKm := startMeters / 1000.0
	endKm := endMeters / 1000.0
	
	// Round to 1 decimal place
	startKm = math.Round(startKm*10) / 10
	endKm = math.Round(endKm*10) / 10
	
	return fmt.Sprintf("%.1f-%.1f km", startKm, endKm)
}

