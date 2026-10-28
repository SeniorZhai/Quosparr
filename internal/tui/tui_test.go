package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

func TestRenderFloatingData(t *testing.T) {
	view := runway.FloatingView{Providers: []runway.FloatingProvider{
		{Name: "Codex", LiveText: "42 tok/s", Meters: []runway.FloatingMeter{
			{Label: "5h", RemainingPercent: 73, Reset: "2h 18m"},
			{Label: "Weekly", RemainingPercent: 28},
		}},
		{Name: "Claude", Meters: []runway.FloatingMeter{{Label: "Balance", ValueText: "$12.34"}}},
		{Name: "Grok", Status: "Log in"},
	}}
	got := renderDashboard(l10n.New(l10n.English), View{Floating: view}, 80, 24, 0, time.Now(), false)
	for _, want := range []string{"Codex", "42 tok/s", "73%", "28%", "Reset in 2h 18m", "$12.34", "Grok", "Log in", "r Refresh", "q/Esc Quit"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "0%") {
		t.Fatal("balance must not appear as a quota percentage")
	}
}

func TestFitTerminalCellsAndControls(t *testing.T) {
	for _, tc := range []struct {
		input string
		cells int
		want  string
	}{
		{"剩余额度", 5, "剩余"},
		{"日本語", 4, "日本"},
		{"한글 ABC", 5, "한글 "},
		{"e\u0301cole", 3, "e\u0301co"},
		{"👩‍💻next", 4, "👩💻"},
		{"a\x1b[2J\n\r\t\x00b\u202ec", 100, "a[2Jbc"},
		{"中文", 1, ""},
		{"abc", 0, ""},
	} {
		if got := fit(tc.input, tc.cells); got != tc.want {
			t.Errorf("fit(%q, %d) = %q, want %q", tc.input, tc.cells, got, tc.want)
		}
	}
}

func TestRenderStaysInsideSmallTerminals(t *testing.T) {
	view := runway.FloatingView{Providers: []runway.FloatingProvider{
		{Name: "Codex\x1b[?1049l", LiveText: "12 tok/s", Meters: []runway.FloatingMeter{
			{Label: "本周", RemainingPercent: 150, Reset: "2 天 3 小时"},
			{Label: "5h", RemainingPercent: -2},
		}},
	}}
	for _, dimensions := range [][2]int{{80, 24}, {20, 8}, {7, 3}, {3, 1}, {1, 1}, {0, 0}} {
		w, h := dimensions[0], dimensions[1]
		got := renderDashboard(l10n.New(l10n.SimplifiedChinese), View{Floating: view}, w, h, 0, time.Now(), true)
		if strings.Contains(got, "\x1b[?1049l") {
			t.Fatal("provider text injected an escape sequence")
		}
		if got == "" {
			continue
		}
		lines := strings.Split(got, "\r\n")
		if len(lines) > h {
			t.Fatalf("%dx%d rendered %d rows", w, h, len(lines))
		}
		for _, line := range lines {
			line = regexp.MustCompile(`\[[0-9;]*m`).ReplaceAllString(line, "")
			if fit(line, w-1) != line {
				t.Errorf("%dx%d row overflows: %q", w, h, line)
			}
		}
	}
	got := renderDashboard(l10n.New(l10n.English), View{Floating: view}, 80, 24, 0, time.Now(), false)
	if !strings.Contains(got, "100%") || !strings.Contains(got, "0%") {
		t.Fatal("quota percentages are not clamped to the display range")
	}
}

type fakeTerminal struct {
	keys      [][]byte
	readError error
	sizeError error
	restored  bool
}

func (t *fakeTerminal) read() ([]byte, error) {
	if len(t.keys) == 0 {
		return nil, t.readError
	}
	key := t.keys[0]
	t.keys = t.keys[1:]
	return key, nil
}
func (t *fakeTerminal) size() (int, int, error) { return 80, 24, t.sizeError }
func (t *fakeTerminal) restore() error          { t.restored = true; return nil }

func TestRunRefreshAndRestoresOnKeyboardExit(t *testing.T) {
	for _, quit := range []byte{'q', 'Q', 3, 27} {
		term := &fakeTerminal{keys: [][]byte{[]byte("\x1b[A"), []byte("r"), {quit}}}
		var output bytes.Buffer
		refreshes, snapshots := 0, 0
		err := run(context.Background(), &output, term, l10n.New(l10n.English), func() View {
			snapshots++
			return View{}
		}, func() { refreshes++ })
		if err != nil || !term.restored || refreshes != 1 || snapshots != 2 {
			t.Fatalf("quit=%d err=%v restored=%v refreshes=%d snapshots=%d", quit, err, term.restored, refreshes, snapshots)
		}
		if !strings.HasPrefix(output.String(), enterScreen) || !strings.HasSuffix(output.String(), leaveScreen) {
			t.Fatal("alternate screen and cursor were not restored")
		}
	}
}

func TestRunRestoresAfterErrorsAndCancellation(t *testing.T) {
	for _, kind := range []string{"read", "size", "write", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			term := &fakeTerminal{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var output bytes.Buffer
			var writer io.Writer = &output
			sentinel := errors.New("terminal unavailable")
			switch kind {
			case "read":
				term.readError = sentinel
			case "size":
				term.sizeError = sentinel
			case "write":
				writer = failedWriter{sentinel}
			case "cancel":
				cancel()
			}
			err := run(ctx, writer, term, l10n.New(l10n.English), func() View { return View{} }, nil)
			if !term.restored {
				t.Fatal("terminal modes were not restored")
			}
			if kind == "cancel" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, sentinel) {
				t.Fatalf("lost original error: %v", err)
			}
			if kind != "write" && !strings.HasSuffix(output.String(), leaveScreen) {
				t.Fatal("terminal screen was not restored")
			}
		})
	}
}

type failedWriter struct{ err error }

func (w failedWriter) Write([]byte) (int, error) { return 0, w.err }

func TestDashboardPagesAndRefreshStatus(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	view := View{LastRefresh: now.Add(-time.Minute), NextRefresh: now.Add(47 * time.Second), IntervalSeconds: 300}
	for _, name := range []string{"Codex", "Claude", "Grok", "Kimi", "DeepSeek", "Copilot"} {
		view.Floating.Providers = append(view.Floating.Providers, runway.FloatingProvider{Name: name, Status: "Sign in"})
	}
	l := l10n.New(l10n.English)
	first := renderDashboard(l, view, 80, 24, 0, now, false)
	second := renderDashboard(l, view, 80, 24, 1, now, false)
	for _, want := range []string{"Auto refresh 5m0s", "Next refresh 00:47", "Page 1/2", "Codex", "Kimi"} {
		if !strings.Contains(first, want) {
			t.Errorf("first page lacks %q", want)
		}
	}
	if strings.Contains(first, "Copilot") || !strings.Contains(second, "Copilot") {
		t.Fatal("providers are not paginated")
	}
	view.Refreshing = true
	if got := renderDashboard(l, view, 80, 24, 0, now, false); !strings.Contains(got, "Refreshing") || strings.Contains(got, "Next refresh 00:47") {
		t.Fatal("refresh state is ambiguous")
	}
	term := &fakeTerminal{keys: [][]byte{[]byte("c")}}
	var out bytes.Buffer
	if err := run(context.Background(), &out, term, l, func() View { return view }, nil); !errors.Is(err, ErrConfigure) || !term.restored {
		t.Fatalf("configure did not restore terminal: %v", err)
	}
}

func TestRunRejectsPipesWithoutOutput(t *testing.T) {
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer writer.Close()
	out, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	err = Run(context.Background(), input, out, l10n.New(l10n.SimplifiedChinese), func() View {
		t.Fatal("non-terminal input must not access the snapshot")
		return View{}
	}, nil)
	if !errors.Is(err, ErrNotTerminal) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "可交互") {
		t.Fatalf("unlocalized error: %v", err)
	}
	info, _ := out.Stat()
	if info.Size() != 0 {
		t.Fatal("escape sequences written to a redirected file")
	}
}
