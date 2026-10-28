package claude

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFetchUsageSSH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake SSH uses a POSIX shell")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, tc := range []struct {
		name, body string
		want       error
	}{
		{"success", `printf '%s' '{"five_hour":{"utilization":25,"resets_at":"2026-10-29T00:00:00Z"}}'`, nil},
		{"unauthorized", "exit 20", ErrUnauthorized},
		{"invalid", "printf 'private-response'", ErrUsageParse},
		{"empty", "printf '{}'", ErrUsageParse},
		{"ssh failure", "echo secret-token >&2; exit 255", ErrUsageSSH},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check := `[ "$*" = '-T -o BatchMode=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 -o ForwardAgent=no -o ClearAllForwardings=yes -- macmini python3 -' ] || exit 99
`
			if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte("#!/bin/sh\n"+check+tc.body+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			snap, err := FetchUsageSSH(context.Background(), "macmini")
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if tc.want == nil && (len(snap.Windows) != 1 || *snap.Windows[0].UsedPercent != 25 || snap.FetchedAt.IsZero()) {
				t.Fatal("incorrect snapshot")
			}
		})
	}
	for _, host := range []string{"", "-oProxyCommand=bad", "host\ncommand", "host;command", "host with space"} {
		if _, err := FetchUsageSSH(context.Background(), host); !errors.Is(err, ErrUsageSSH) {
			t.Fatal("unsafe host accepted")
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte("#!/bin/sh\nexec sleep 10\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := FetchUsageSSH(ctx, "macmini"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestSSHUsageScript(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not installed")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(`{"claudeAiOauth":{"accessToken":"fixture-secret"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"success", `{"five_hour":{"utilization":12,"resets_at":"2026-10-29T00:00:00Z","token":"must-not-return"},"email":"must-not-return"}`, 0},
		{"invalid percent", `{"five_hour":{"utilization":101}}`, 21},
		{"invalid reset", `{"five_hour":{"utilization":12,"resets_at":"must-not-return"}}`, 21},
		{"unknown", `{"email":"must-not-return"}`, 21},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bootstrap := `import io, os, urllib.request
class FakeOpener:
    def open(self, request, timeout):
        assert request.get_header("Authorization") == "Bearer fixture-secret"
        assert request.full_url == "https://api.anthropic.com/api/oauth/usage"
        return io.BytesIO(os.environ["TEST_USAGE"].encode())
urllib.request.build_opener = lambda *args: FakeOpener()
`
			cmd := exec.Command(python, "-")
			cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+dir, "TEST_USAGE="+tc.body)
			cmd.Stdin = strings.NewReader(bootstrap + sshUsageScript)
			out, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tc.code {
				t.Fatalf("exit = %d, want %d", code, tc.code)
			}
			if strings.Contains(string(out), "must-not-return") || strings.Contains(string(out), "fixture-secret") {
				t.Fatal("private fields escaped remote boundary")
			}
			if code == 0 {
				snap, err := ParseUsageSnapshot(out)
				if err != nil || len(snap.Windows) != 1 || *snap.Windows[0].UsedPercent != 12 {
					t.Fatal("invalid sanitized output")
				}
			}
		})
	}
}
