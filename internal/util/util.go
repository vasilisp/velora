package util

import (
	"fmt"
	"os"
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
