package service

import (
	"strconv"
	"time"
)

func targetTimeFromString(timeStr string) (time.Time, bool) {
	// parse time in format "HH:MM:SS"
	parsedTime, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		return time.Time{}, false
	}
	// calculate minTime as duration until the parsed time
	now := time.Now()
	targetTime := time.Date(now.Year(), now.Month(), now.Day(), parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second(), 0, now.Location())
	if targetTime.Before(now) {
		targetTime = targetTime.Add(24 * time.Hour)
	}
	return targetTime, true
}

func parseFloat(value string) float64 {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0.0
	}

	return f
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}
