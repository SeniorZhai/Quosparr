package tuiinstall

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/licoy/quosparr/internal/platform"
)

func TestInstallIndependentCommand(t *testing.T) {
	module := t.TempDir()
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module github.com/licoy/quosparr\n\ngo 1.27.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(module, "cmd", "quosparr-tui")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(module, "quosparr-tui")
	cmd := exec.Command("go", "build", "-o", source, "./cmd/quosparr-tui")
	cmd.Dir = module
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, output)
	}
	want, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"fresh", "upgrade", "wrapper", "conflict", "symlink", "invalid-source"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			if runtime.GOOS == "windows" {
				t.Setenv("LOCALAPPDATA", home)
			}
			destination := Destination(home)
			if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
				t.Fatal(err)
			}
			var old []byte
			src := source
			switch kind {
			case "upgrade":
				old = want
			case "wrapper":
				old = []byte(previousWrapper + "\n")
			case "conflict", "invalid-source":
				old = []byte("unrelated command")
			case "symlink":
				if runtime.GOOS == "windows" {
					t.Skip("symlink creation requires a Windows policy not assumed by this test")
				}
				if err := os.Symlink(source, destination); err != nil {
					t.Fatal(err)
				}
			}
			if old != nil {
				if err := os.WriteFile(destination, old, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "invalid-source" {
				src = destination
			}
			path, err := Install(home, src)
			if kind == "conflict" || kind == "symlink" || kind == "invalid-source" {
				wantErr := ErrConflict
				if kind == "invalid-source" {
					wantErr = ErrSource
				}
				if !errors.Is(err, wantErr) {
					t.Fatalf("error = %v; want %v", err, wantErr)
				}
				if kind == "symlink" {
					if _, err := os.Readlink(destination); err != nil {
						t.Fatal("link overwritten")
					}
					return
				}
				got, _ := os.ReadFile(destination)
				if !bytes.Equal(got, old) {
					t.Fatal("old command changed on failure")
				}
				return
			}
			if err != nil || path != destination {
				t.Fatalf("install = %q, %v", path, err)
			}
			got, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatal("installed command differs from bundled executable")
			}
			if err := platform.VerifyOwnerOnly(destination, 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(home, ".quosparr")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("installation touched configuration")
			}
		})
	}
}
