package usage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanDailyDeduplicatesFilesAndEventsAndUsesUTCDays(t *testing.T) {
	home := t.TempDir()
	data := strings.Join([]string{
		`{"timestamp":"2026-10-27T23:30:00-07:00","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":5},"total_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":5}}}}`,
		`{"timestamp":"2026-10-27T23:30:01-07:00","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":5},"total_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":5}}}}`,
		`{"timestamp":"2026-10-27T00:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":0,"output_tokens":0}}}}`,
		`{"timestamp":"2026-10-26T23:59:59Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":999,"output_tokens":999}}}}`,
	}, "\n") + "\n"
	for _, root := range []string{"sessions", "archived_sessions"} {
		path := filepath.Join(home, root, "same.jsonl")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	scanner := Scanner{CodexHome: home, Location: time.UTC, Cache: &CacheScanCache{}}
	window := Window{Start: time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)}
	for range 2 {
		report, err := scanner.ScanDaily(context.Background(), window)
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Days) != 2 || report.Days["2026-10-28"] != 105 || report.UnreadLines != 0 {
			t.Fatalf("daily usage = %+v", report)
		}
		if _, present := report.Days["2026-10-27"]; !present {
			t.Fatal("recorded zero was discarded")
		}
		if _, present := report.Days["2026-10-26"]; present {
			t.Fatal("outside-window day included")
		}
	}
}

func TestScanDailyReportsUnreadLinesAndCancellation(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "sessions")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"timestamp":"2026-10-28T00:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":4,"output_tokens":2}}}}` + "\n" + `{"type":"event_msg","payload":{"type":"token_count","info":` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "one.jsonl"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	window := Window{Start: time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)}
	scanner := Scanner{CodexHome: home, Location: time.UTC}
	report, err := scanner.ScanDaily(context.Background(), window)
	if err != nil || report.Days["2026-10-28"] != 6 || report.UnreadLines != 1 {
		t.Fatalf("partial report = %+v, %v", report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := scanner.ScanDaily(ctx, window); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}
