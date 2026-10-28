package livespeed

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestReadGrokOpenTurnUsesRecentMeasuredSpeed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "cwd", "sess")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	end := now.Add(-time.Minute)
	updates := `{"timestamp":` + itoa(end.Unix()) + `,"params":{"update":{"sessionUpdate":"turn_completed","usage":{"outputTokens":100,"apiDurationMs":2000}}}}` + "\n"
	start := now.Add(-4 * time.Second).Format(time.RFC3339Nano)
	events := `{"ts":"` + start + `","type":"turn_started","model_id":"grok-test"}` + "\n" +
		`{"ts":"` + start + `","type":"phase_changed","phase":"waiting_for_model"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "updates.jsonl"), []byte(updates), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(events), 0o600); err != nil {
		t.Fatal(err)
	}
	responses, flights := Read(now, Homes{Grok: home})
	if len(responses) != 1 || len(flights) != 1 {
		t.Fatalf("responses %d flights %d", len(responses), len(flights))
	}
	if got := OutputRate(now, responses, flights); mathAbs(got-50) > 1 {
		t.Fatalf("rate = %v", got)
	}
}

func TestReadIgnoresAFinishedSession(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "cwd", "sess")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	end := now.Add(-10 * time.Minute)
	body := `{"timestamp":` + itoa(end.Unix()) + `,"params":{"update":{"sessionUpdate":"turn_completed","usage":{"outputTokens":9000,"apiDurationMs":3000}}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "updates.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	responses, flights := Read(now, Homes{Grok: home})
	if len(flights) != 0 {
		t.Fatalf("flights = %d", len(flights))
	}
	if got := OutputRate(now, responses, flights); got != 0 {
		t.Fatalf("finished session rate = %v", got)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
