package util

import (
	"encoding/json"
	"fmt"
	"time"
)

type Weekday time.Weekday

const (
	Monday    Weekday = Weekday(time.Monday)
	Tuesday   Weekday = Weekday(time.Tuesday)
	Wednesday Weekday = Weekday(time.Wednesday)
	Thursday  Weekday = Weekday(time.Thursday)
	Friday    Weekday = Weekday(time.Friday)
	Saturday  Weekday = Weekday(time.Saturday)
	Sunday    Weekday = Weekday(time.Sunday)
)

func ParseWeekday(s string) (Weekday, error) {
	for i := time.Sunday; i <= time.Saturday; i++ {
		if i.String() == s {
			return Weekday(i), nil
		}
	}

	return 0, fmt.Errorf("invalid weekday: %q", s)
}

func (d Weekday) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Weekday(d).String())
}

func (d *Weekday) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	day, err := ParseWeekday(s)
	if err != nil {
		return err
	}
	*d = day

	return nil
}

func (d Weekday) String() string {
	return time.Weekday(d).String()
}

var AllDays = []Weekday{
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
