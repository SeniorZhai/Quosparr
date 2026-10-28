// Package tuiinstall copies the bundled console program into a user command directory.
package tuiinstall

import (
	"bytes"
	"debug/buildinfo"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/licoy/quosparr/internal/platform"
)

var (
	ErrSource   = errors.New("bundled TUI is unavailable or invalid")
	ErrConflict = errors.New("another command already exists at the installation path")
	ErrInstall  = errors.New("could not install TUI")
)

const commandPackage = "github.com/licoy/quosparr/cmd/quosparr-tui"
const previousWrapper = "#!/bin/sh\nexec \"/Applications/Quosparr.app/Contents/MacOS/Quosparr\" \"$@\""

func BundledPath(executable string) string {
	dir := filepath.Dir(executable)
	if runtime.GOOS == "darwin" {
		dir = filepath.Join(dir, "..", "Resources")
	}
	name := "quosparr-tui"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}

func Destination(home string) string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "Programs", "QuosparrTUI", "quosparr.exe")
	}
	return filepath.Join(home, ".local", "bin", "quosparr")
}

// Install never runs the old command and never changes shell configuration.
func Install(home, source string) (string, error) {
	data, err := readRegular(source)
	if err != nil || !isCommand(data, true) {
		return "", ErrSource
	}
	destination := Destination(home)
	old, err := readRegular(destination)
	if err == nil {
		if !isCommand(old, false) && strings.TrimSpace(string(old)) != previousWrapper {
			return "", ErrConflict
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", ErrConflict
	}
	if err := platform.WriteFileAtomic(destination, data, 0o700); err != nil {
		return "", ErrInstall
	}
	return destination, nil
}

func isCommand(data []byte, currentPlatform bool) bool {
	info, err := buildinfo.Read(bytes.NewReader(data))
	if err != nil || info.Path != commandPackage {
		return false
	}
	if !currentPlatform {
		return true
	}
	var system, arch string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "GOOS":
			system = setting.Value
		case "GOARCH":
			arch = setting.Value
		}
	}
	return system == runtime.GOOS && arch == runtime.GOARCH
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 128<<20 {
		return nil, ErrSource
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrSource
	}
	data, err := io.ReadAll(io.LimitReader(f, (128<<20)+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != info.Size() {
		return nil, ErrSource
	}
	return data, nil
}
