//go:build !darwin && !linux && !windows

package tui

import "os"

func checkTerminal(_, _ *os.File) error            { return ErrNotTerminal }
func openTerminal(_, _ *os.File) (terminal, error) { return nil, ErrNotTerminal }
