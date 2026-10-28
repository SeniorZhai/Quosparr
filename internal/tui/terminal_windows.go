package tui

import (
	"encoding/binary"
	"errors"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var readConsoleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputW")

type console struct {
	in, out               *os.File
	inputMode, outputMode uint32
}

func checkTerminal(in, out *os.File) error {
	for _, f := range []*os.File{in, out} {
		var mode uint32
		if err := windows.GetConsoleMode(windows.Handle(f.Fd()), &mode); err != nil {
			return ErrNotTerminal
		}
	}
	return nil
}

func openTerminal(in, out *os.File) (terminal, error) {
	t := &console{in: in, out: out}
	if err := windows.GetConsoleMode(windows.Handle(in.Fd()), &t.inputMode); err != nil {
		return nil, err
	}
	if err := windows.GetConsoleMode(windows.Handle(out.Fd()), &t.outputMode); err != nil {
		return nil, err
	}
	input := t.inputMode &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_QUICK_EDIT_MODE | windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	input |= windows.ENABLE_EXTENDED_FLAGS | windows.ENABLE_WINDOW_INPUT
	if err := windows.SetConsoleMode(windows.Handle(in.Fd()), input); err != nil {
		return nil, err
	}
	if err := windows.SetConsoleMode(windows.Handle(out.Fd()), t.outputMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING|windows.ENABLE_PROCESSED_OUTPUT); err != nil {
		return nil, errors.Join(err, windows.SetConsoleMode(windows.Handle(in.Fd()), t.inputMode))
	}
	return t, nil
}

func (t *console) restore() error {
	return errors.Join(windows.SetConsoleMode(windows.Handle(t.in.Fd()), t.inputMode), windows.SetConsoleMode(windows.Handle(t.out.Fd()), t.outputMode))
}

func (t *console) size() (int, int, error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(t.out.Fd()), &info); err != nil {
		return 0, 0, err
	}
	return int(info.Window.Right-info.Window.Left) + 1, int(info.Window.Bottom-info.Window.Top) + 1, nil
}

func (t *console) read() ([]byte, error) {
	ready, err := windows.WaitForSingleObject(windows.Handle(t.in.Fd()), 100)
	if err != nil || ready == uint32(windows.WAIT_TIMEOUT) {
		return nil, err
	}
	// INPUT_RECORD is 20 bytes; KEY_EVENT_RECORD starts at byte 4.
	var record [20]byte
	var count uint32
	ok, _, callErr := readConsoleInput.Call(t.in.Fd(), uintptr(unsafe.Pointer(&record[0])), 1, uintptr(unsafe.Pointer(&count)))
	if ok == 0 {
		if callErr == syscall.Errno(0) {
			callErr = syscall.EINVAL
		}
		return nil, callErr
	}
	if count == 0 || binary.LittleEndian.Uint16(record[:2]) != 1 || binary.LittleEndian.Uint32(record[4:8]) == 0 {
		return nil, nil
	}
	key := binary.LittleEndian.Uint16(record[14:16])
	if key > 127 {
		return nil, nil
	}
	return []byte{byte(key)}, nil
}
