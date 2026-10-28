package livespeed

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMonitorGrokKeepsLastCompletedRateDuringLongFirstTurn(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	home := t.TempDir()
	past := filepath.Join(home, "sessions", "cwd", "past")
	active := filepath.Join(home, "sessions", "cwd", "active")
	for _, dir := range []string{past, active} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, body string, at time.Time) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	end := now.Add(-3 * time.Hour)
	completion := `{"timestamp":` + itoa(end.Unix()) + `,"params":{"update":{"sessionUpdate":"turn_completed","usage":{"outputTokens":100,"apiDurationMs":2000}}}}` + "\n"
	write(filepath.Join(past, "updates.jsonl"), completion, end)
	events := `{"ts":"` + now.Add(-time.Second).Format(time.RFC3339Nano) + `","type":"phase_changed","phase":"streaming_text"}` + "\n"
	write(filepath.Join(active, "events.jsonl"), events, now)
	m := &Monitor{homes: Homes{Grok: home}, ctx: context.Background()}
	m.sample(now)
	check := func() {
		t.Helper()
		parts := m.Current()
		if len(parts) != 1 || parts[0].Pending || !parts[0].Last || parts[0].Text != "50 tok/s" {
			t.Fatalf("long first turn lost the completed rate: %+v", parts)
		}
	}
	check()
	// Later output can push the usage record beyond the bounded tail.
	write(filepath.Join(past, "updates.jsonl"), completion+strings.Repeat("{}\n", tailBytes/3+1), now)
	m.sample(now.Add(time.Minute))
	check()
	write(filepath.Join(active, "events.jsonl"), events+`{"ts":"`+now.Add(time.Minute).Format(time.RFC3339Nano)+`","type":"turn_ended"}`+"\n", now)
	m.sample(now.Add(2 * time.Minute))
	if parts := m.Current(); len(parts) != 0 {
		t.Fatalf("idle turn kept a speed: %+v", parts)
	}
	later := now.Add(24 * time.Hour)
	events = `{"ts":"` + later.Add(-time.Second).Format(time.RFC3339Nano) + `","type":"phase_changed","phase":"streaming_text"}` + "\n"
	write(filepath.Join(active, "events.jsonl"), events, later)
	m.sample(later)
	if parts := m.Current(); len(parts) != 1 || !parts[0].Pending || parts[0].Text != "" {
		t.Fatalf("expired history still displayed: %+v", parts)
	}
}
