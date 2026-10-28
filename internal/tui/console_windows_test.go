package tui

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPrepareConsolePreservesRedirectedOutput(t *testing.T) {
	old, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(t.TempDir(), "redirect")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, windows.Handle(file.Fd())); err != nil {
		t.Fatal(err)
	}
	defer windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, old)
	restore, err := PrepareConsole()
	if restore != nil {
		defer restore()
	}
	if !errors.Is(err, ErrNotTerminal) {
		t.Fatalf("redirected output should be refused: %v", err)
	}
	current, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || current != windows.Handle(file.Fd()) {
		t.Fatal("redirected output was replaced")
	}
}
