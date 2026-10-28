package livespeed

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestReadGrokOpenTurnLabelsTheLastMeasuredSpeed(t *testing.T) {
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
	if got := Parts(now, responses, flights); len(got) != 1 || got[0].Text != "50 tok/s" || !got[0].Last {
		t.Fatalf("rate = %+v", got)
	}
}

func TestGrokCompletedLongMultiModelTurnUsesWholeAPIDuration(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, output := range []string{`"outputTokens":60000,`, ""} {
		updates := `{"timestamp":` + itoa(now.Unix()) + `,"params":{"update":{"sessionUpdate":"turn_completed","usage":{` + output + `"apiDurationMs":1200000,"modelUsage":{"a":{"outputTokens":20000},"b":{"outputTokens":40000}}}}}}` + "\n"
		var responses []Response
		var flights []Flight
		parseGrok(now, "session", []byte(updates), nil, &responses, &flights)
		parts := Parts(now, responses, flights)
		if len(responses) != 1 || len(parts) != 1 || parts[0].Text != "50 tok/s" || !parts[0].Last {
			t.Fatalf("whole turn rate: responses=%+v, parts=%+v", responses, parts)
		}
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

func TestReadSkipsEmptyCodexRoot(t *testing.T) {
	home := t.TempDir()
	t.Chdir(home)
	if err := os.Mkdir("sessions", 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	body := `{"timestamp":"` + now.Add(-time.Second).Format(time.RFC3339Nano) + `","type":"event_msg","payload":{"type":"task_started"}}` + "\n"
	if err := os.WriteFile(filepath.Join("sessions", "session.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, flights := Read(now, Homes{Codex: home}); len(flights) != 1 {
		t.Fatal("fixture should contain a live Codex request")
	}
	if responses, flights := Read(now, Homes{}); len(responses) != 0 || len(flights) != 0 {
		t.Fatal("empty root read sessions relative to the working directory")
	}
}

func TestReadGrokPendingWithoutCompletedUsage(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	start := `{"ts":"` + now.Add(-4*time.Second).Format(time.RFC3339Nano) + `","type":"turn_started"}` + "\n"
	stamp := now.Add(-time.Second).Format(time.RFC3339Nano)
	activity := `{"ts":"` + stamp + `","type":"phase_changed","phase":"streaming_text"}` + "\n"
	ended := `{"ts":"` + stamp + `","type":"turn_ended"}` + "\n"
	cleanup := `{"ts":"` + stamp + `","type":"tool_completed"}` + "\n" +
		`{"ts":"` + stamp + `","type":"mcp_init_completed"}` + "\n"
	for _, tc := range []struct {
		name, events string
		pending      bool
	}{
		{"first turn", start + activity, true},
		{"truncated text", start + strings.Repeat("{}\n", tailBytes/3+1) + activity, true},
		{"truncated reasoning", strings.ReplaceAll(activity, "streaming_text", "streaming_reasoning"), true},
		{"truncated first token", `{"ts":"` + stamp + `","type":"first_token"}` + "\n", true},
		{"ended then cleanup", activity + ended + cleanup, false},
		{"running tools", start + activity + `{"ts":"` + stamp + `","type":"phase_changed","phase":"tool_execution"}` + "\n", false},
		{"permission prompt", start + activity + `{"ts":"` + stamp + `","type":"phase_changed","phase":"permission_prompt"}` + "\n", false},
		{"cleanup only", cleanup, false},
		{"stale activity", strings.ReplaceAll(activity, stamp, now.Add(-flightFresh-time.Second).Format(time.RFC3339Nano)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, "sessions", "cwd", "sess")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(tc.events), 0o600); err != nil {
				t.Fatal(err)
			}
			responses, flights := Read(now, Homes{Grok: home})
			parts := Parts(now, responses, flights)
			if len(responses) != 0 {
				t.Fatalf("made up %d usage records", len(responses))
			}
			if tc.pending {
				if len(flights) != 1 || len(parts) != 1 || parts[0].Source != "grok" || !parts[0].Pending || parts[0].Text != "" {
					t.Fatalf("flights = %d, parts = %+v", len(flights), parts)
				}
			} else if len(flights) != 0 || len(parts) != 0 {
				t.Fatalf("inactive session: flights = %d, parts = %+v", len(flights), parts)
			}
		})
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
