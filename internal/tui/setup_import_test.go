package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/store"
)

func TestAppImportCopiesOnlySelectedNecessaryCredentialFields(t *testing.T) {
	for _, tc := range []struct {
		provider                             store.Provider
		index, activeKey, auth, want, absent string
	}{
		{store.ProviderCodex, "index.json", "activeAccountId", `{"auth_mode":"chatgpt","tokens":{"access_token":"selected-access","id_token":"selected-id","account_id":"fixture-account","refresh_token":"DROP_REFRESH","extra":"DROP_UNKNOWN"},"unrelated":"DROP_OTHER"}`, "selected-access", "DROP_"},
		{store.ProviderClaude, "claude-index.json", "currentAccountID", `{"claudeAiOauth":{"accessToken":"selected-access","refreshToken":"DROP_REFRESH","expiresAt":1893456000000,"scopes":["user:profile"],"extra":"DROP_UNKNOWN"},"mcpOAuth":{"secret":"DROP_MCP"}}`, "selected-access", "DROP_"},
		{store.ProviderGrok, "grok-index.json", "currentAccountID", `{"https://auth.x.ai::fixture":{"key":"selected-access","email":"fixture@example.invalid","expires_at":"2030-01-01T00:00:00Z","refresh_token":"DROP_REFRESH","extra":"DROP_UNKNOWN"},"xai::api_key":{"key":"DROP_OTHER"}}`, "selected-access", "DROP_"},
	} {
		t.Run(string(tc.provider), func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(platform.RunwayDir(home), "accounts")
			index := []byte(`{"` + tc.activeKey + `":"current","accounts":[{"id":"current"},{"id":"other"}]}`)
			indexPath, authPath := filepath.Join(root, tc.index), filepath.Join(root, "current", "auth.json")
			setupFixtureFile(t, indexPath, index)
			setupFixtureFile(t, authPath, []byte(tc.auth))
			setupFixtureFile(t, filepath.Join(root, "other", "auth.json"), []byte(`{"apiKey":"DROP_NONCURRENT"}`))
			account, data, err := importAppCredential(home, tc.provider)
			if err != nil {
				t.Fatal(err)
			}
			if account.Mode != "imported" || account.CredentialID != "" || !bytes.Contains(data, []byte(tc.want)) || bytes.Contains(data, []byte(tc.absent)) {
				t.Fatal("import did not keep only the selected credential's necessary fields")
			}
			if got, _ := os.ReadFile(indexPath); !bytes.Equal(got, index) {
				t.Fatal("App index changed")
			}
			if got, _ := os.ReadFile(authPath); string(got) != tc.auth {
				t.Fatal("App auth changed")
			}
		})
	}
}

func TestAppAPIImportPreservesSiteWithoutCopyingMetadataOrOtherKeys(t *testing.T) {
	for _, p := range []store.Provider{store.ProviderCodex, store.ProviderClaude, store.ProviderKimi} {
		t.Run(string(p), func(t *testing.T) {
			home := t.TempDir()
			root := platform.RunwayDir(home)
			switch p {
			case store.ProviderCodex:
				setupFixtureFile(t, filepath.Join(root, "accounts", "index.json"), []byte(`{"activeAccountId":"current","accounts":[{"id":"current","apiBaseURL":"https://relay.example.invalid/v1","apiModel":"unused-model"}]}`))
				setupFixtureFile(t, filepath.Join(root, "accounts", "current", "auth.json"), []byte(`{"auth_mode":"apikey","OPENAI_API_KEY":"selected-api-key","unrelated":"DROP_SECRET"}`))
			case store.ProviderClaude:
				setupFixtureFile(t, filepath.Join(root, "accounts", "claude-index.json"), []byte(`{"currentAccountID":"current","accounts":[{"id":"current","kind":"api","api":{"baseURL":"https://relay.example.invalid/v1","model":"unused-model"}}]}`))
				setupFixtureFile(t, filepath.Join(root, "accounts", "current", "auth.json"), []byte(`{"apiKey":"selected-api-key","unrelated":"DROP_SECRET"}`))
			case store.ProviderKimi:
				setupFixtureFile(t, filepath.Join(root, "remote-quota.json"), []byte(`{"kimiKey":"selected-api-key","kimiRegion":"intl","deepseekKey":"DROP_SECRET"}`))
			}
			account, data, err := importAppCredential(home, p)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]string
			if json.Unmarshal(data, &fields) != nil || len(fields) != 1 || fields["apiKey"] != "selected-api-key" {
				t.Fatal("API import retained fields other than the selected key")
			}
			if p == store.ProviderCodex && account.BaseURL != "https://relay.example.invalid/v1" {
				t.Fatal("Codex site lost")
			}
			if p == store.ProviderClaude && account.BaseURL != "https://relay.example.invalid" {
				t.Fatal("Claude site not normalized")
			}
			if p == store.ProviderKimi && account.Region != "intl" {
				t.Fatal("Kimi region lost")
			}
		})
	}
}

func TestImportFailureDoesNotDiscardOtherSelectedProvider(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(platform.RunwayDir(home), "accounts")
	setupFixtureFile(t, filepath.Join(root, "index.json"), []byte(`{"activeAccountId":"current","accounts":[{"id":"current"}]}`))
	setupFixtureFile(t, filepath.Join(root, "current", "auth.json"), []byte(`{"auth_mode":"apikey","OPENAI_API_KEY":"selected-api-key"}`))
	var out bytes.Buffer
	term := &fakeTerminal{keys: [][]byte{[]byte("1,2\n300\n1\n3\n1\n")}}
	cfg, err := configure(context.Background(), &out, term, home, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Accounts[store.ProviderCodex].Mode != "imported" || cfg.Accounts[store.ProviderClaude].Mode != "cli" {
		t.Fatal("partial import discarded successful or previous provider config")
	}
	if !strings.Contains(out.String(), "Import ready") || !strings.Contains(out.String(), "No usable current App account") {
		t.Fatal("partial results not reported")
	}
	if strings.Contains(out.String(), "selected-api-key") {
		t.Fatal("import displayed a key")
	}
}

func TestImportRejectsIndexPathTraversal(t *testing.T) {
	home := t.TempDir()
	setupFixtureFile(t, filepath.Join(platform.RunwayDir(home), "accounts", "index.json"), []byte(`{"activeAccountId":"../../outside","accounts":[{"id":"../../outside"}]}`))
	if _, _, err := importAppCredential(home, store.ProviderCodex); err == nil {
		t.Fatal("accepted index path traversal")
	}
}
