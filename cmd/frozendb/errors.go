package main

import (
	"fmt"
	"os"
)

// formatError formats a FrozenDBError for CLI output.
// Format: "Error: message"
// Per FR-007, all errors must follow this format where message is err.Error().
func formatError(err error) string {
	return fmt.Sprintf("Error: %s", err.Error())
}

// printError prints an error to stderr in structured format and exits with code 1.
// Per FR-007, all errors must go to stderr with exit code 1.
// Per FR-005, success exits with code 0 (handled by caller, not this function).
func printError(err error) {
	fmt.Fprintln(os.Stderr, formatError(err))
	os.Exit(1)
}
