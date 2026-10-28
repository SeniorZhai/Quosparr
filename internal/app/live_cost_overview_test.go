package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/sessions"
	"github.com/licoy/quosparr/internal/sqliteruntime"
	"github.com/licoy/quosparr/internal/usage"
)

func TestCostOverviewScansAllProvidersWithoutLogin(t *testing.T) {
	for _, indexed := range []bool{false, true} {
		t.Run(fmt.Sprintf("indexed=%v", indexed), func(t *testing.T) {
			rt, roots := sessionTestRuntime(t)
			if indexed {
				if err := sqliteruntime.Load(); err != nil {
					t.Skipf("SQLite unavailable: %v", err)
				}
				repo, err := usage.NewRepository(usage.Config{CodexHome: rt.codexFolder(), DatabasePath: filepath.Join(t.TempDir(), "usage.sqlite3"), Location: time.Local})
				if err != nil {
					t.Fatal(err)
				}
				rt.repo = repo
				t.Cleanup(func() { _ = repo.Close() })
			}
			now := time.Now()
			today := usage.Today(now, time.Local).Window.Start
			stamp := today.Add(now.Sub(today) / 2)
			var codexLog, claudeLog, grokLog strings.Builder
			for i, days := range []int{0, -6, -7, -29, -30} {
				at := stamp.AddDate(0, 0, days)
				tokens := (i + 1) * 10
				fmt.Fprintf(&codexLog, `{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d,"cached_input_tokens":0,"output_tokens":0}}},"turn_context":{"model":"gpt-5.5"}}`+"\n", at.Format(time.RFC3339Nano), tokens)
				fmt.Fprintf(&claudeLog, `{"type":"assistant","sessionId":"fixture","timestamp":%q,"message":{"id":"msg-%d","model":"claude-haiku-4-5","usage":{"input_tokens":%d}}}`+"\n", at.Format(time.RFC3339Nano), i, tokens)
				fmt.Fprintf(&grokLog, `{"timestamp":%d,"params":{"update":{"sessionUpdate":"turn_completed","usage":{"inputTokens":%d,"outputTokens":0}}}}`+"\n", at.Unix(), tokens)
			}
			files := map[string]string{
				filepath.Join(roots[sessions.ProviderCodex][0], "rollout-overview.jsonl"):    codexLog.String(),
				filepath.Join(roots[sessions.ProviderClaude][0], "project", "fixture.jsonl"): claudeLog.String(),
				filepath.Join(roots[sessions.ProviderGrok][0], "project", "summary.json"):    `{"info":{"id":"fixture"},"current_model_id":"grok-4.5"}`,
				filepath.Join(roots[sessions.ProviderGrok][0], "project", "updates.jsonl"):   grokLog.String(),
			}
			for path, content := range files {
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := rt.queryCostOverview(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for i, want := range []int64{30, 90, 300} {
				period := got.Periods[i]
				if period.Tokens != want || period.EstimatedUSD == nil || period.EstimatedUSD.Sign() <= 0 || period.Partial {
					t.Fatalf("period %d = %+v, want %d tokens with a complete price", i, period, want)
				}
			}
		})
	}
}

func TestCostOverviewKeepsSuccessfulProvidersWhenOneFails(t *testing.T) {
	rt, roots := sessionTestRuntime(t)
	path := filepath.Join(roots[sessions.ProviderClaude][0], "project", "fixture.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	at := usage.Today(time.Now(), time.Local).Window.Start
	data := fmt.Sprintf(`{"type":"assistant","sessionId":"fixture","timestamp":%q,"message":{"id":"one","model":"claude-haiku-4-5","usage":{"input_tokens":10}}}`+"\n", at.Format(time.RFC3339Nano))
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	root := roots[sessions.ProviderGrok][0]
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := rt.queryCostOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, period := range got.Periods {
		if !period.Partial || period.EstimatedUSD == nil || period.EstimatedUSD.Sign() <= 0 || period.Tokens != 10 {
			t.Fatalf("successful provider was lost after another source failed: %+v", period)
		}
	}
	if _, err := (&liveApp{}).queryCostOverview(context.Background()); !errors.Is(err, errDetailUnavailable) {
		t.Fatalf("all failed = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := rt.queryCostOverview(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled = %v", err)
	}
}

func TestCostOverviewDoesNotCapGrokSessions(t *testing.T) {
	rt, roots := sessionTestRuntime(t)
	at := usage.Today(time.Now(), time.Local).Window.Start
	for i := range 401 {
		dir := filepath.Join(roots[sessions.ProviderGrok][0], fmt.Sprintf("session-%d", i))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string]string{
			"summary.json":  fmt.Sprintf(`{"info":{"id":"session-%d"},"current_model_id":"grok-4.5"}`, i),
			"updates.jsonl": fmt.Sprintf(`{"timestamp":%d,"params":{"update":{"sessionUpdate":"turn_completed","usage":{"inputTokens":1}}}}`+"\n", at.Unix()),
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := rt.queryCostOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, period := range got.Periods {
		if period.Tokens != 401 || period.Partial || period.EstimatedUSD == nil {
			t.Fatalf("session total was capped: %+v", period)
		}
	}
}
