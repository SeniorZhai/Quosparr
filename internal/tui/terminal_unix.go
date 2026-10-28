//go:build darwin || linux

package tui

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

type console struct {
	in, out  *os.File
	original unix.Termios
}

func checkTerminal(in, out *os.File) error {
	for _, f := range []*os.File{in, out} {
		if _, err := unix.IoctlGetTermios(int(f.Fd()), getTermios); err != nil {
			return ErrNotTerminal
		}
	}
	return nil
}

func openTerminal(in, out *os.File) (terminal, error) {
	original, err := unix.IoctlGetTermios(int(in.Fd()), getTermios)
	if err != nil {
		return nil, err
	}
	raw := *original
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN], raw.Cc[unix.VTIME] = 0, 0
	if err := unix.IoctlSetTermios(int(in.Fd()), setTermios, &raw); err != nil {
		return nil, err
	}
	return &console{in: in, out: out, original: *original}, nil
}

func (t *console) restore() error {
	return unix.IoctlSetTermios(int(t.in.Fd()), setTermios, &t.original)
}

func (t *console) size() (int, int, error) {
	v, err := unix.IoctlGetWinsize(int(t.out.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}
	return int(v.Col), int(v.Row), nil
}

func (t *console) read() ([]byte, error) {
	fds := []unix.PollFd{{Fd: int32(t.in.Fd()), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, 100)
	if errors.Is(err, unix.EINTR) {
		return nil, nil
	}
	if err != nil || n == 0 {
		return nil, err
	}
	if fds[0].Revents&(unix.POLLNVAL|unix.POLLERR) != 0 {
		return nil, unix.EIO
	}
	buf := make([]byte, 64)
	n, err = unix.Read(int(t.in.Fd()), buf)
	if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
		return nil, nil
	}
	if err == nil && n == 0 {
		return nil, io.EOF
	}
	return buf[:max(0, n)], err
}
