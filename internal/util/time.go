package util

import (
	"fmt"
	"time"
)

var AllDays = []time.Weekday{
	time.Monday,
	time.Tuesday,
	time.Wednesday,
	time.Thursday,
	time.Friday,
	time.Saturday,
	time.Sunday,
}

func FormatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// BeginningOfWeek returns the time corresponding to Monday of the current week at 00:00.
func BeginningOfWeek(t time.Time) time.Time {
	// Normalize to local time zone if needed
	year, month, day := t.Date()
	location := t.Location()
	t = time.Date(year, month, day, 0, 0, 0, 0, location)

	// Calculate how many days to subtract to get back to Monday
	weekday := int(t.Weekday())
	if weekday == 0 { // Sunday
		weekday = 7
	}

	startOfWeek := t.AddDate(0, 0, -weekday+1)
	return startOfWeek
}
