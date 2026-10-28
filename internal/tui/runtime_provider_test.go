package tui

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/netx"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/store"
)

type runtimeTransport func(*http.Request) (*http.Response, error)

func TestRuntimeReadsAdvertisedClaudeUpstreamWindows(t *testing.T) {
	home := t.TempDir()
	account := runtimeCredential(t, home, "claude-api-tui-", `{"apiKey":"fake-relay-key"}`)
	account.BaseURL = "https://relay.example"
	ctx := runtimeNetwork(t, func(r *http.Request) (*http.Response, error) {
		code, body := 404, "{}"
		switch r.URL.Path {
		case "/":
			if r.Header.Get("Authorization") != "" {
				t.Error("public metadata received the key")
			}
			code, body = 200, `{"apis":["/v1/magpie/quotas"]}`
		case "/v1/magpie/quotas":
			if r.Header.Get("Authorization") != "Bearer fake-relay-key" {
				t.Error("upstream quota missing the configured key")
			}
			code, body = 200, `{"data":[{"kind":"subscription","provider":"gateway-claude","readAt":"2026-08-21T12:00:00Z","windows":[{"name":"5 hour","used":42,"remaining":58,"resetsAt":"2026-08-21T16:00:00Z"},{"name":"7 day","used":21,"remaining":79}]}]}`
		}
		return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	reading := readProvider(ctx, home, store.ProviderClaude, account, l10n.New(l10n.English), time.Now())
	if len(reading.meters) != 2 || reading.meters[0].RemainingPercent != 58 || reading.meters[1].RemainingPercent != 79 || reading.meters[0].Balance || reading.meters[0].ValueText != "" {
		t.Fatalf("upstream windows not shared with TUI: %+v, %s", reading.meters, reading.status)
	}
}

func (f runtimeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func runtimeNetwork(t *testing.T, handler runtimeTransport) context.Context {
	t.Helper()
	network := netx.SystemContext()
	network.Client(netx.Standard).Transport = handler
	network.Client(netx.WithoutCookies).Transport = handler
	t.Cleanup(network.CloseIdleConnections)
	return netx.WithContext(context.Background(), network)
}

func runtimeCredential(t *testing.T, home, prefix, body string) Account {
	t.Helper()
	id := prefix + strings.Repeat("0", 32)
	path := filepath.Join(platform.RunwayDir(home), "accounts", id, "auth.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return Account{Mode: "imported", CredentialID: id}
}

func TestRuntimeExplainsMissingAPIBalanceWithoutCallingItLoginFailure(t *testing.T) {
	ctx := runtimeNetwork(t, func(r *http.Request) (*http.Response, error) {
		body, code := `{}`, 404
		if strings.HasSuffix(r.URL.Path, "/models") {
			body, code = `{"data":[{"id":"test-model"}]}`, 200
		}
		return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	lc := l10n.New(l10n.SimplifiedChinese)
	got := readAPIQuota(ctx, "https://relay.example/v1", "fixture-api-key", store.ProviderClaude, lc, time.Now())
	if len(got.meters) != 0 || got.status != lc.TUI("balanceUnsupported") {
		t.Fatalf("unsupported API balance status = %q", got.status)
	}
}

func TestRuntimeRejectsExpiredImportedCredentialsWithoutRefreshing(t *testing.T) {
	now := time.Now()
	jwt := "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, now.Add(-time.Minute).Unix()))) + ".test"
	ctx := runtimeNetwork(t, func(*http.Request) (*http.Response, error) {
		t.Error("expired credential made a network request")
		return nil, context.Canceled
	})
	for _, tc := range []struct {
		provider              store.Provider
		prefix, body, command string
	}{
		{store.ProviderCodex, "tui-codex-", fmt.Sprintf(`{"tokens":{"access_token":%q,"refresh_token":"fake-refresh-token-long-enough"}}`, jwt), "codex login"},
		{store.ProviderClaude, "claude-tui-", fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"fake-access","refreshToken":"fake-refresh","expiresAt":%d}}`, now.Add(-time.Minute).UnixMilli()), "claude"},
		{store.ProviderGrok, "grok-tui-", fmt.Sprintf(`{"https://auth.x.ai::test":{"user_id":"fake-user","key":%q,"refresh_token":"fake-refresh","expires_at":%q}}`, jwt, now.Add(-time.Minute).UTC().Format(time.RFC3339)), "grok login --oauth"},
	} {
		t.Run(string(tc.provider), func(t *testing.T) {
			home := t.TempDir()
			account := runtimeCredential(t, home, tc.prefix, tc.body)
			lc := l10n.New(l10n.English)
			reading := readProvider(ctx, home, tc.provider, account, lc, now)
			if len(reading.meters) != 0 || reading.status != fmt.Sprintf(lc.TUI("expiredImported"), tc.command) {
				t.Fatalf("expired imported credential status = %q", reading.status)
			}
			data, err := ReadCredential(home, account)
			if err != nil || string(data) != tc.body {
				t.Fatal("read changed the private credential")
			}
			for _, name := range []string{"index.json", "claude-index.json", "grok-index.json"} {
				if _, err := os.Stat(filepath.Join(platform.RunwayDir(home), "accounts", name)); !os.IsNotExist(err) {
					t.Fatal("read created a managed account index")
				}
			}
		})
	}
}

func TestRuntimeGrokUsesRotatedOfficialTokenOnlyForTheImportedAccount(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	credential := func(user string, expiry time.Time) string {
		jwt := "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, expiry.Unix()))) + ".test"
		return fmt.Sprintf(`{"https://auth.x.ai::test":{"user_id":%q,"team_id":"fixture-team","key":%q,"expires_at":%q}}`, user, jwt, expiry.Format(time.RFC3339))
	}
	for _, tc := range []struct {
		name, officialUser string
		expired            bool
		wantQuota          bool
		officialState      string
	}{
		{"same account refreshed", "fixture-user", false, true, "valid"},
		{"different official account", "another-user", false, false, "valid"},
		{"official still expired", "fixture-user", true, false, "valid"},
		{"official missing", "fixture-user", false, false, "missing"},
		{"official invalid", "fixture-user", false, false, "invalid"},
		{"official without token", "fixture-user", false, false, "no token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			officialDir := filepath.Join(home, "grok")
			t.Setenv("GROK_HOME", officialDir)
			imported := credential("fixture-user", now.Add(-time.Minute))
			account := runtimeCredential(t, home, "grok-tui-", imported)
			expiry := now.Add(time.Hour)
			if tc.expired {
				expiry = now.Add(-time.Second)
			}
			official := credential(tc.officialUser, expiry)
			if tc.officialState == "invalid" {
				official = "{invalid"
			} else if tc.officialState == "no token" {
				official = `{"https://auth.x.ai::test":{"user_id":"fixture-user","team_id":"fixture-team"}}`
			}
			if err := os.MkdirAll(officialDir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(officialDir, "auth.json")
			if tc.officialState != "missing" {
				if err := os.WriteFile(path, []byte(official), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var requests atomic.Int32
			ctx := runtimeNetwork(t, func(r *http.Request) (*http.Response, error) {
				requests.Add(1)
				if !tc.wantQuota {
					t.Error("unrelated or expired credential made a request")
				}
				if strings.Contains(r.URL.Path, "oauth") {
					t.Error("read-only TUI attempted token refresh")
				}
				fresh := "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, expiry.Unix()))) + ".test"
				if r.Header.Get("Authorization") != "Bearer "+fresh {
					t.Error("quota did not use the rotated token")
				}
				body, code := `{"config":{"creditUsagePercent":25,"currentPeriod":{"type":"USAGE_PERIOD_TYPE_WEEKLY"}}}`, 200
				if r.URL.Host == "grok.com" || strings.HasSuffix(r.URL.Path, "/settings") {
					body, code = "", 204
				}
				return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			lc := l10n.New(l10n.English)
			reading := readProvider(ctx, home, store.ProviderGrok, account, lc, now)
			if tc.wantQuota {
				if requests.Load() == 0 || len(reading.meters) != 1 || reading.meters[0].RemainingPercent != 75 {
					t.Fatalf("rotated official login was rejected: meters=%+v status=%q", reading.meters, reading.status)
				}
			} else if requests.Load() != 0 || reading.status != fmt.Sprintf(lc.TUI("expiredImported"), "grok login --oauth") {
				t.Fatalf("expired imported status=%q requests=%d", reading.status, requests.Load())
			}
			got, err := ReadCredential(home, account)
			if err != nil || string(got) != imported {
				t.Fatal("reading changed the imported credential")
			}
			got, err = os.ReadFile(path)
			if tc.officialState == "missing" {
				if !os.IsNotExist(err) {
					t.Fatal("reading restored missing official credentials")
				}
			} else if err != nil || string(got) != official {
				t.Fatal("reading changed official credentials")
			}
		})
	}
}

func TestRuntimeUnconfiguredDoesNotReadAppCredentials(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(platform.RunwayDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(platform.RunwayDir(home), "remote-quota.json"), []byte(`{"deepSeekKey":"app-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := runtimeNetwork(t, func(*http.Request) (*http.Response, error) {
		t.Error("unconfigured provider made a network request")
		return nil, context.Canceled
	})
	lc := l10n.New(l10n.English)
	for _, p := range store.Providers() {
		if got := readProvider(ctx, home, p, Account{}, lc, time.Now()); got.status != lc.TUI("unconfigured") {
			t.Errorf("%s = %q", p, got.status)
		}
	}
}

func TestRuntimeCLIReadsPreserveOfficialFiles(t *testing.T) {
	home := t.TempDir()
	codexDir, claudeDir := filepath.Join(home, "codex"), filepath.Join(home, "claude")
	t.Setenv("CODEX_HOME", codexDir)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	files := map[string]string{
		filepath.Join(codexDir, "auth.json"):      `{"auth_mode":"apikey","OPENAI_API_KEY":"fake-relay-key"}`,
		filepath.Join(codexDir, "config.toml"):    "model_provider = \"custom\"\n[model_providers.custom]\nbase_url = \"https://relay.example/v1\"\n",
		filepath.Join(claudeDir, "settings.json"): `{"env":{"ANTHROPIC_BASE_URL":"https://relay.example/v1","ANTHROPIC_AUTH_TOKEN":"fake-relay-key"},"unrelated":true}`,
	}
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx := runtimeNetwork(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "relay.example" {
			t.Errorf("key sent to unexpected host %s", r.URL.Host)
		}
		body, code := "{}", 404
		switch r.URL.Path {
		case "/api/usage/token":
			if r.Header.Get("Authorization") != "Bearer fake-relay-key" {
				t.Error("missing relay credential")
			}
			body, code = `{"code":true,"data":{"total_available":750000,"total_used":250000,"total_granted":1000000,"unlimited_quota":false}}`, 200
		case "/api/status":
			body, code = `{"success":true,"data":{"quota_per_unit":500000,"quota_display_type":"USD"}}`, 200
		}
		return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	for _, p := range []store.Provider{store.ProviderCodex, store.ProviderClaude} {
		reading := readProvider(ctx, home, p, Account{Mode: "cli"}, l10n.New(l10n.English), time.Now())
		if len(reading.meters) != 1 || reading.meters[0].ValueText != "$1.50" || reading.meters[0].RemainingPercent != 75 {
			t.Fatalf("%s balance = %+v, %s", p, reading.meters, reading.status)
		}
	}
	for path, want := range files {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatal("runtime changed an official file")
		}
	}
	if _, err := os.Stat(platform.RunwayDir(home)); !os.IsNotExist(err) {
		t.Fatal("read-only CLI runtime created App data")
	}
}

func TestRuntimeQuotaReadsUseIndependentCredentials(t *testing.T) {
	now := time.Now()
	jwt := "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, now.Add(time.Hour).Unix()))) + ".test"
	for _, tc := range []struct {
		provider                           store.Provider
		prefix, credential, host, response string
		want                               int
	}{
		{store.ProviderCodex, "tui-codex-", fmt.Sprintf(`{"tokens":{"access_token":%q}}`, jwt), "chatgpt.com", `{"rate_limit":{"primary_window":{"used_percent":25,"limit_window_seconds":18000,"reset_at":1893456000}}}`, 75},
		{store.ProviderClaude, "claude-tui-", fmt.Sprintf(`{"claudeAiOauth":{"accessToken":"fake-access","expiresAt":%d}}`, now.Add(time.Hour).UnixMilli()), "api.anthropic.com", `{"five_hour":{"utilization":25},"seven_day":{"utilization":50}}`, 75},
		{store.ProviderGrok, "grok-tui-", fmt.Sprintf(`{"https://auth.x.ai::test":{"user_id":"fake-user","key":%q,"expires_at":%q}}`, jwt, now.Add(time.Hour).UTC().Format(time.RFC3339)), "cli-chat-proxy.grok.com", `{"config":{"creditUsagePercent":25,"currentPeriod":{"type":"USAGE_PERIOD_TYPE_WEEKLY"}}}`, 75},
		{store.ProviderKimi, "tui-kimi-", `{"apiKey":"fake-kimi"}`, "api.kimi.com", `{"usages":{"limit_5h":{"used_ratio":0.25}}}`, 75},
		{store.ProviderDeepSeek, "tui-deepseek-", `{"apiKey":"fake-deepseek"}`, "api.deepseek.com", `{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":"3.50","granted_balance":"0","topped_up_balance":"3.50"}]}`, 100},
		{store.ProviderCopilot, "tui-copilot-", `{"apiKey":"fake-copilot"}`, "api.github.com", `{"quota_snapshots":{"premium_interactions":{"percent_remaining":75,"entitlement":100,"remaining":75}}}`, 75},
	} {
		t.Run(string(tc.provider), func(t *testing.T) {
			home := t.TempDir()
			account := runtimeCredential(t, home, tc.prefix, tc.credential)
			var requests atomic.Int32
			ctx := runtimeNetwork(t, func(r *http.Request) (*http.Response, error) {
				requests.Add(1)
				if r.Header.Get("Authorization") == "" {
					t.Error("missing credential header")
				}
				response, code := tc.response, 200
				if tc.provider == store.ProviderGrok && (r.URL.Host == "grok.com" || strings.HasSuffix(r.URL.Path, "/settings")) {
					response, code = "", 204
				} else if r.URL.Host != tc.host {
					t.Errorf("unexpected quota host %s", r.URL.Host)
				}
				return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response)), Request: r}, nil
			})
			reading := readProvider(ctx, home, tc.provider, account, l10n.New(l10n.English), now)
			if requests.Load() == 0 || len(reading.meters) == 0 || reading.meters[0].RemainingPercent != tc.want {
				t.Fatalf("quota = %+v, status %q, requests %d", reading.meters, reading.status, requests.Load())
			}
			if tc.provider == store.ProviderDeepSeek && (!reading.meters[0].Balance || reading.meters[0].ValueText != "$3.50") {
				t.Fatalf("balance = %+v", reading.meters[0])
			}
		})
	}
}
