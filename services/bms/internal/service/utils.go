package service

import (
	"context"
	"reflect"
	"strconv"
	"strings"
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
	return strconv.FormatFloat(value, 'f', 4, 64)
}

func parseTimestamp(value string) (*time.Time, bool) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, false
	}

	return &t, true
}

func formatTimestamp(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func parseFloatDuration(value string) time.Duration {
	f := parseFloat(value)
	// convert float to duration in seconds
	return time.Duration(f * float64(time.Minute))
}

func formatFloatDuration(d time.Duration) string {
	// convert duration to float in minutes
	f := d.Minutes()
	return formatFloat(f)
}

func getPopulatedJSONFields(v interface{}) []string {
	var fieldNames []string
	val := reflect.ValueOf(v)

	// Handle pointers
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Ensure we are working with a struct
	if val.Kind() != reflect.Struct {
		return fieldNames
	}

	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		fieldVal := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported (private) fields or zero-value fields
		if !fieldType.IsExported() || fieldVal.IsZero() {
			continue
		}

		// Get the json tag
		jsonTag := fieldType.Tag.Get("json")

		// Skip fields explicitly ignored in JSON
		if jsonTag == "-" {
			continue
		}

		// Parse tag name (e.g., "email_address,omitempty" -> "email_address")
		tagName := strings.Split(jsonTag, ",")[0]

		// Fallback to struct field name if no json tag is defined
		if tagName == "" {
			tagName = fieldType.Name
		}

		fieldNames = append(fieldNames, tagName)
	}

	return fieldNames
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
