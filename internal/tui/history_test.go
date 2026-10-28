package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/store"
)

func TestHistoryReadsAllLocalProvidersWithoutWritingAppOrOfficialData(t *testing.T) {
	home := t.TempDir()
	codexDir, claudeDir, grokDir := filepath.Join(home, "codex"), filepath.Join(home, "claude"), filepath.Join(home, "grok")
	t.Setenv("CODEX_HOME", codexDir)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("GROK_HOME", grokDir)
	at := time.Date(2026, 10, 28, 6, 30, 0, 0, time.UTC)
	files := map[string]string{
		filepath.Join(codexDir, "sessions", "one.jsonl"):                   `{"timestamp":"2026-10-27T23:30:00-07:00","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":15}}}}` + "\n",
		filepath.Join(claudeDir, "projects", "demo", "one.jsonl"):          `{"type":"assistant","sessionId":"one","timestamp":"2026-10-27T23:30:00-07:00","message":{"id":"msg1","model":"claude-sonnet-4-5","usage":{"input_tokens":20,"cache_read_input_tokens":30,"cache_creation_input_tokens":40,"output_tokens":10}}}` + "\n",
		filepath.Join(grokDir, "sessions", "demo", "one", "summary.json"):  `{"info":{"id":"one","cwd":"/fixture"},"updated_at":"2026-10-28T06:30:00Z","current_model_id":"grok-4.5"}`,
		filepath.Join(grokDir, "sessions", "demo", "one", "updates.jsonl"): fmt.Sprintf(`{"timestamp":%d,"params":{"update":{"sessionUpdate":"turn_completed","usage":{"inputTokens":100,"cachedReadTokens":30,"outputTokens":20,"totalTokens":120}}}}`, at.Unix()) + "\n",
	}
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	reader := newHistoryReader(home)
	now := at.Add(time.Hour)
	for p, want := range map[store.Provider]int64{store.ProviderCodex: 115, store.ProviderClaude: 100, store.ProviderGrok: 120} {
		got := reader.Read(context.Background(), p, now)
		if got.Failed || got.Incomplete || !got.Available || !reflect.DeepEqual(got.Days, map[string]int64{"2026-10-28": want}) {
			t.Fatalf("%s history = %+v", p, got)
		}
		if got.From != time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) || got.Through != time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC) {
			t.Fatalf("incorrect UTC range: %+v", got)
		}
	}
	for path, want := range files {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatal("scan changed official log")
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".quosparr")); !os.IsNotExist(err) {
		t.Fatal("scan created App data")
	}
}

func TestHistoryUsesUTCYearBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, now, from, through string
	}{
		{"year boundary", "2026-01-31T12:00:00Z", "2026-01-01", "2026-01-31"},
		{"leap month", "2024-02-29T12:00:00Z", "2024-01-01", "2024-02-29"},
		{"UTC before local month", "2026-03-01T00:30:00+08:00", "2026-01-01", "2026-02-28"},
		{"UTC after local month", "2025-12-31T23:30:00-08:00", "2026-01-01", "2026-01-01"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			codexDir := filepath.Join(home, "codex")
			t.Setenv("CODEX_HOME", codexDir)
			now, err := time.Parse(time.RFC3339, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			from, err := time.Parse(time.DateOnly, tc.from)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(codexDir, "sessions", "one.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			var data string
			for _, at := range []time.Time{from.Add(-time.Second), from, now.Add(-time.Second), now.Add(time.Second)} {
				data += fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`+"\n", at.Format(time.RFC3339))
			}
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			got := newHistoryReader(home).Read(context.Background(), store.ProviderCodex, now)
			wantDays := map[string]int64{tc.from: 1}
			wantDays[tc.through]++
			if got.Failed || got.From != from || got.Through.Format(time.DateOnly) != tc.through || got.Through.Location() != time.UTC || !reflect.DeepEqual(got.Days, wantDays) {
				t.Fatalf("month range = %+v, want %s through %s with days %v", got, tc.from, tc.through, wantDays)
			}
		})
	}
}

func TestHistoryDistinguishesNoLogsFailureAndPartialData(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, "codex")
	t.Setenv("CODEX_HOME", codexDir)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, "claude"))
	t.Setenv("GROK_HOME", filepath.Join(home, "grok"))
	reader := newHistoryReader(home)
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, p := range []store.Provider{store.ProviderCodex, store.ProviderClaude, store.ProviderGrok, store.ProviderKimi} {
		got := reader.Read(context.Background(), p, now)
		if got.Available || got.Failed || len(got.Days) != 0 {
			t.Fatalf("missing logs = %+v", got)
		}
	}
	path := filepath.Join(codexDir, "sessions", "one.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"timestamp":"2026-10-28T00:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}` + "\n" + `{"type":"event_msg","payload":{"type":"token_count","info":` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	got := reader.Read(context.Background(), store.ProviderCodex, now)
	if !got.Available || !got.Incomplete || got.Failed || got.Days["2026-10-28"] != 1 {
		t.Fatalf("partial = %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got = reader.Read(ctx, store.ProviderCodex, now)
	if !got.Failed || got.Available || len(got.Days) != 0 {
		t.Fatalf("failure = %+v", got)
	}
}
