package util

import "time"

// BeginningOfWeek returns the time corresponding to Monday of the current week at 00:00.
func BeginningOfWeek(t time.Time) time.Time {
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
