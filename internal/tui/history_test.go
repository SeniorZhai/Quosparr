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
		if got.Through.Sub(got.From) != 370*24*time.Hour || got.From.Location() != time.UTC {
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
