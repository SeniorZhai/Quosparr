package tui

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	consoleKernel = windows.NewLazySystemDLL("kernel32.dll")
	attachConsole = consoleKernel.NewProc("AttachConsole")
	freeConsole   = consoleKernel.NewProc("FreeConsole")
)

// PrepareConsole attaches a GUI-subsystem build to its launching terminal.
// Explicit input/output redirection is rejected rather than overwritten.
func PrepareConsole() (restore func() error, err error) {
	ids := []uint32{windows.STD_INPUT_HANDLE, windows.STD_OUTPUT_HANDLE, windows.STD_ERROR_HANDLE}
	files := []**os.File{&os.Stdin, &os.Stdout, &os.Stderr}
	var oldHandles [3]windows.Handle
	var oldFiles [3]*os.File
	var missing [3]bool
	for i, id := range ids {
		h, getErr := windows.GetStdHandle(id)
		if getErr != nil && !errors.Is(getErr, windows.ERROR_INVALID_HANDLE) {
			return nil, getErr
		}
		oldHandles[i], oldFiles[i] = h, *files[i]
		missing[i] = h == 0 || h == windows.InvalidHandle
		if !missing[i] {
			_, typeErr := windows.GetFileType(h)
			missing[i] = errors.Is(typeErr, windows.ERROR_INVALID_HANDLE)
			if typeErr != nil && !missing[i] {
				return nil, typeErr
			}
		}
		if i < 2 && !missing[i] {
			var mode uint32
			if windows.GetConsoleMode(h, &mode) != nil {
				return nil, ErrNotTerminal
			}
		}
	}
	if !missing[0] && !missing[1] && !missing[2] {
		return func() error { return nil }, nil
	}
	attached := false
	if ok, _, attachErr := attachConsole.Call(uintptr(^uint32(0))); ok != 0 {
		attached = true
	} else if !errors.Is(attachErr, windows.ERROR_ACCESS_DENIED) {
		return nil, errors.Join(ErrNotTerminal, attachErr)
	}
	var opened []*os.File
	restore = func() error {
		var restoreErr error
		for i, id := range ids {
			*files[i] = oldFiles[i]
			restoreErr = errors.Join(restoreErr, windows.SetStdHandle(id, oldHandles[i]))
		}
		for _, f := range opened {
			restoreErr = errors.Join(restoreErr, f.Close())
		}
		if attached {
			if ok, _, freeErr := freeConsole.Call(); ok == 0 {
				if freeErr == syscall.Errno(0) {
					freeErr = syscall.EINVAL
				}
				restoreErr = errors.Join(restoreErr, freeErr)
			}
		}
		return restoreErr
	}
	for i, id := range ids {
		h := oldHandles[i]
		if missing[i] {
			name := "CONOUT$"
			if i == 0 {
				name = "CONIN$"
			}
			f, openErr := os.OpenFile(name, os.O_RDWR, 0)
			if openErr != nil {
				return nil, errors.Join(openErr, restore())
			}
			opened = append(opened, f)
			*files[i] = f
			h = windows.Handle(f.Fd())
		}
		// AttachConsole may reset inherited handles, including redirected stderr.
		if setErr := windows.SetStdHandle(id, h); setErr != nil {
			return nil, errors.Join(setErr, restore())
		}
	}
	return restore, nil
}
