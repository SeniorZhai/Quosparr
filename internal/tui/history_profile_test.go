package tui

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/store"
)

func historyProfileTestToken(now time.Time, subject string) string {
	return "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d,"sub":%q}`, now.Add(time.Hour).Unix(), subject))) + ".test"
}

func TestHistoryProfileUsesSelectedCredentialWithoutWritingOrAddingSeries(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, mode := range []string{"cli", "imported"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			codexDir := filepath.Join(home, "codex")
			t.Setenv("CODEX_HOME", codexDir)
			officialToken, importedToken := historyProfileTestToken(now, "official"), historyProfileTestToken(now, "imported")
			official := fmt.Sprintf(`{"tokens":{"access_token":%q,"account_id":"official-account"}}`, officialToken)
			imported := fmt.Sprintf(`{"tokens":{"access_token":%q,"account_id":"imported-account"}}`, importedToken)
			account := runtimeCredential(t, home, "tui-codex-", imported)
			wantToken, wantAccount := importedToken, "imported-account"
			if mode == "cli" {
				account = Account{Mode: "cli"}
				wantToken, wantAccount = officialToken, "official-account"
			}
			files := map[string]string{
				filepath.Join(codexDir, "auth.json"): official,
				filepath.Join(home, ".quosparr", "accounts", "tui-codex-"+strings.Repeat("0", 32), "auth.json"): imported,
				filepath.Join(codexDir, "sessions", "fixture.jsonl"):                                            `{"timestamp":"2026-10-28T01:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":5}}}}` + "\n",
			}
			for path, body := range files {
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			ctx := runtimeNetwork(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet || r.URL.Host != "chatgpt.com" || r.URL.Path != "/backend-api/wham/profiles/me" {
					t.Error("history must only request the official profile endpoint")
				}
				if r.Header.Get("Authorization") != "Bearer "+wantToken || r.Header.Get("ChatGPT-Account-Id") != wantAccount {
					t.Error("history did not use the selected credential")
				}
				if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) > 30*time.Second {
					t.Error("profile request is not bounded by the runtime timeout")
				}
				body := `{"stats":{"daily_usage_buckets":[{"start_date":"2025-12-31","tokens":999},{"start_date":"2026-01-01","tokens":100},{"start_date":"2026-10-27","tokens":9000000000},{"start_date":"2026-10-28","tokens":0},{"start_date":"2026-10-29","tokens":999}]},"metadata":{"stats_as_of":"2026-10-27"}}`
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			got := newHistoryReader(home, account).Read(ctx, store.ProviderCodex, now)
			if calls != 1 || got.Failed || got.OfficialFailed || got.Incomplete || !got.Available || got.OfficialStatsAsOf != "2026-10-27" {
				t.Fatalf("profile history status: calls=%d history=%+v", calls, got)
			}
			if !reflect.DeepEqual(got.Days, map[string]int64{"2026-10-28": 15}) || !reflect.DeepEqual(got.AllDevices, map[string]int64{"2026-01-01": 100, "2026-10-27": 9000000000, "2026-10-28": 0}) {
				t.Fatalf("local and official series must stay separate and clipped to the UTC year: %+v", got)
			}
			for path, want := range files {
				if data, err := os.ReadFile(path); err != nil || string(data) != want {
					t.Error("history changed a credential or session file")
				}
			}
			if err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() {
					if _, ok := files[path]; !ok {
						t.Error("history created an unexpected file")
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHistoryProfileSkipsAPIAndRejectsUnavailableOAuthWithoutFallback(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	expired := historyProfileTestToken(now.Add(-2*time.Hour), "expired")
	valid := fmt.Sprintf(`{"tokens":{"access_token":%q}}`, historyProfileTestToken(now, "valid"))
	for _, tc := range []struct {
		name, mode, imported, official string
		failed                         bool
	}{
		{"unconfigured", "", "", valid, false},
		{"manual api", "api", `{"apiKey":"fixture-api-key"}`, valid, false},
		{"imported api wrapper", "imported", `{"apiKey":"fixture-api-key"}`, valid, false},
		{"imported official api", "imported", `{"auth_mode":"apikey","OPENAI_API_KEY":"fixture-api-key"}`, valid, false},
		{"cli api", "cli", valid, `{"auth_mode":"apikey","OPENAI_API_KEY":"fixture-api-key"}`, false},
		{"expired imported", "imported", fmt.Sprintf(`{"tokens":{"access_token":%q,"refresh_token":"fixture-refresh-long-enough"}}`, expired), valid, true},
		{"missing imported", "imported", "", valid, true},
		{"invalid imported", "imported", `{}`, valid, true},
		{"missing cli", "cli", valid, "", true},
		{"invalid cli", "cli", valid, `{}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			codexDir := filepath.Join(home, "codex")
			t.Setenv("CODEX_HOME", codexDir)
			account := Account{Mode: tc.mode}
			if tc.imported != "" {
				account = runtimeCredential(t, home, "tui-codex-", tc.imported)
				account.Mode = tc.mode
			}
			if tc.official != "" {
				if err := os.MkdirAll(codexDir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(tc.official), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			ctx := runtimeNetwork(t, func(*http.Request) (*http.Response, error) {
				t.Error("unavailable OAuth or API account made a profile/refresh request")
				return nil, fmt.Errorf("unexpected request")
			})
			got := newHistoryReader(home, account).Read(ctx, store.ProviderCodex, now)
			if got.OfficialFailed != tc.failed || got.Available || got.Failed || len(got.AllDevices) != 0 {
				t.Fatalf("unexpected profile state: %+v", got)
			}
		})
	}
}

func TestHistoryProfileAndLocalFailuresAreIndependent(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name        string
		localFailed bool
		status      int
		body        string
	}{
		{"local failure", true, 200, `{"stats":{"daily_usage_buckets":[{"start_date":"2026-01-01","tokens":100}]}}`},
		{"unauthorized profile", false, 401, `{}`},
		{"unavailable profile", false, 503, `{}`},
		{"malformed profile", false, 200, `{"stats":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			codexDir := filepath.Join(home, "codex")
			t.Setenv("CODEX_HOME", codexDir)
			account := runtimeCredential(t, home, "tui-codex-", fmt.Sprintf(`{"tokens":{"access_token":%q}}`, historyProfileTestToken(now, "valid")))
			path, body := filepath.Join(codexDir, "sessions", "fixture.jsonl"), `{"timestamp":"2026-10-28T01:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10}}}}`+"\n"
			if tc.localFailed {
				path, body = filepath.Join(codexDir, "sessions"), "not a directory"
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			ctx := runtimeNetwork(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Path != "/backend-api/wham/profiles/me" {
					t.Error("history attempted a request other than the profile")
				}
				return &http.Response{StatusCode: tc.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body)), Request: r}, nil
			})
			got := newHistoryReader(home, account).Read(ctx, store.ProviderCodex, now)
			if calls != 1 || !got.Available || got.Failed != tc.localFailed || got.OfficialFailed == tc.localFailed {
				t.Fatalf("one failed source must not hide the other: calls=%d history=%+v", calls, got)
			}
			if tc.localFailed && (len(got.Days) != 0 || got.AllDevices["2026-01-01"] != 100) || !tc.localFailed && (len(got.AllDevices) != 0 || got.Days["2026-10-28"] != 10) {
				t.Fatalf("successful history source was lost: %+v", got)
			}
		})
	}
}
