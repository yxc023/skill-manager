// Package ui provides user-facing output helpers for skill-manager.
// Plain text output — no color dependency. Format conventions match common CLI tools.
package ui

import "fmt"

const (
	check = "✓"
	warn  = "!"
	fail  = "✗"
)

// OK prints a green checkmark + message. (Plain text — no ANSI.)
func OK(msg string) string { return fmt.Sprintf("%s %s", check, msg) }

// Warn prints a yellow warning.
func Warn(msg string) string { return fmt.Sprintf("%s %s", warn, msg) }

// Fail prints a red failure.
func Fail(msg string) string { return fmt.Sprintf("%s %s", fail, msg) }

// Name prints a label for an entity (skill/agent/etc).
func Name(msg string) string { return msg }
