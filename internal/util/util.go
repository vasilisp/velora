package util

import (
	"fmt"
	"os"
	"regexp"
)

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

func SanitizeTerminalOutput(input string) string {
	return sanitizeAnsi.ReplaceAllString(sanitizeControlChars.ReplaceAllString(input, ""), "")
}
