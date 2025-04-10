package util

import (
	"fmt"
	"time"
)

type DayOfWeek int

const (
	Monday DayOfWeek = iota
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
	Sunday
)

func (d DayOfWeek) String() (string, error) {
	switch d {
	case Monday:
		return "Monday", nil
	case Tuesday:
		return "Tuesday", nil
	case Wednesday:
		return "Wednesday", nil
	case Thursday:
		return "Thursday", nil
	case Friday:
		return "Friday", nil
	case Saturday:
		return "Saturday", nil
	case Sunday:
		return "Sunday", nil
	default:
		return "", fmt.Errorf("invalid day of week: %d", d)
	}
}

var AllDays = []DayOfWeek{
	Monday,
	Tuesday,
	Wednesday,
	Thursday,
	Friday,
	Saturday,
	Sunday,
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
