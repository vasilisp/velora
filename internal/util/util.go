package util

import (
	"fmt"
	"os"
	"regexp"
	"unicode"

	"golang.org/x/text/unicode/norm"
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

func Fatalf(format string, v ...any) {
	fmt.Fprintf(os.Stderr, format, v...)
	os.Exit(1)
}

func Assert(condition bool, message string) {
	if !condition {
		Fatalf("assertion failed: %s", message)
	}
}

func FormatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func FormatDistance(meters int) string {
	if meters >= 1000 {
		return fmt.Sprintf("%.1fkm", float64(meters)/1000)
	}
	return fmt.Sprintf("%dm", meters)
}

var sanitizeControlChars = regexp.MustCompile(`[\x00-\x08\x0B-\x1F\x7F]`)

var sanitizeAnsi = regexp.MustCompile(`\x1B\[[0-9;]*[a-zA-Z]`)

func SanitizeOutput(input string, removeNewlines bool) string {
	// Remove ASCII control characters (including optional newlines)
	cleaned := sanitizeControlChars.ReplaceAllString(input, "")

	// Remove ANSI escape sequences
	cleaned = sanitizeAnsi.ReplaceAllString(cleaned, "")

	// Remove non-printable Unicode characters (e.g., C0 control characters)
	var result []rune
	for _, r := range cleaned {
		if r == '\n' {
			if removeNewlines {
				result = append(result, ' ')
				continue
			}
			result = append(result, r)
		} else if unicode.IsPrint(r) || unicode.IsSpace(r) {
			result = append(result, r)
		}
	}

	// Normalize the Unicode string to a consistent form (optional)
	return norm.NFC.String(string(result))
}
