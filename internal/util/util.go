package util

import (
	"fmt"
	"os"
	"strings"
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

func FormatDistance(meters int) string {
	if meters >= 1000 {
		return fmt.Sprintf("%.1fkm", float64(meters)/1000)
	}
	return fmt.Sprintf("%dm", meters)
}

func Capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}
