//go:build !windows

package tui

// PrepareConsole is needed only by Windows builds using the GUI subsystem.
func PrepareConsole() (func() error, error) { return func() error { return nil }, nil }
