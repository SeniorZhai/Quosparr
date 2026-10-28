package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/store"
)

func setupFixtureFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := platform.EnsurePrivateDirectory(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if err := platform.WriteFileAtomic(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func setupTestConfig() Config {
	return Config{Providers: []store.Provider{store.ProviderCodex}, IntervalSeconds: 300, Language: l10n.English, Accounts: map[store.Provider]Account{store.ProviderCodex: {Mode: "cli"}}}
}

func TestConfigKeepsKeysOnlyInPrivateDedicatedCredential(t *testing.T) {
	home := t.TempDir()
	cfg := setupTestConfig()
	cfg.Accounts[store.ProviderCodex] = Account{Mode: "api", BaseURL: "https://example.invalid/v1"}
	secret := []byte(`{"apiKey":"fixture-secret-never-in-config"}`)
	saved, err := saveConfig(home, cfg, map[store.Provider][]byte{store.ProviderCodex: secret})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Accounts[store.ProviderCodex].CredentialID != "" {
		t.Fatal("save mutated the draft account map")
	}
	loaded, err := LoadConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.IntervalSeconds != 300 || loaded.Accounts[store.ProviderCodex] != saved.Accounts[store.ProviderCodex] {
		t.Fatal("configuration did not round-trip")
	}
	data, err := os.ReadFile(ConfigPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("fixture-secret")) || bytes.Contains(data, []byte("apiKey")) {
		t.Fatal("credential in config")
	}
	credential, err := ReadCredential(home, loaded.Accounts[store.ProviderCodex])
	if err != nil || !bytes.Equal(credential, secret) {
		t.Fatalf("credential did not round-trip: %v", err)
	}
	for _, path := range []string{ConfigPath(home), filepath.Join(platform.RunwayDir(home), "accounts", loaded.Accounts[store.ProviderCodex].CredentialID, "auth.json")} {
		if err := platform.VerifyOwnerOnly(path, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := platform.VerifyOwnerOnly(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("official Codex directory changed")
	}
}

func TestConfigCommitFailureRemovesOnlyNewCredentialCopies(t *testing.T) {
	home := t.TempDir()
	root := platform.RunwayDir(home)
	original := filepath.Join(root, "accounts", "existing-app-account", "auth.json")
	setupFixtureFile(t, original, []byte(`{"must":"remain"}`))
	if err := os.MkdirAll(ConfigPath(home), 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(ConfigPath(home), "keep")
	if err := os.WriteFile(marker, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := setupTestConfig()
	cfg.Accounts[store.ProviderCodex] = Account{Mode: "api"}
	_, err := saveConfig(home, cfg, map[store.Provider][]byte{store.ProviderCodex: []byte(`{"apiKey":"fixture-only"}`)})
	if !errors.Is(err, ErrConfigUnavailable) {
		t.Fatalf("expected failed config commit: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "accounts"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "existing-app-account" {
		t.Fatal("rollback kept a new credential or removed existing data")
	}
	if data, err := os.ReadFile(original); err != nil || string(data) != `{"must":"remain"}` {
		t.Fatal("existing credential changed")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "original" {
		t.Fatal("preexisting config path changed")
	}
}

func TestLoadConfigRejectsInvalidValuesWithoutEchoingContent(t *testing.T) {
	for _, data := range []string{
		`{"apiKey":"SECRET"}`,
		`{"providers":["codex"],"intervalSeconds":1,"language":"english","accounts":{}}`,
		`{"providers":["codex","codex"],"intervalSeconds":300,"language":"english","accounts":{}}`,
		`{"providers":["codex"],"intervalSeconds":300,"language":"english","accounts":{"codex":{"mode":"api","credentialID":"../../SECRET"}}}`,
	} {
		t.Run(data[:min(25, len(data))], func(t *testing.T) {
			home := t.TempDir()
			setupFixtureFile(t, ConfigPath(home), []byte(data))
			_, err := LoadConfig(home)
			if !errors.Is(err, ErrConfigInvalid) || strings.Contains(err.Error(), "SECRET") {
				t.Fatalf("unsafe config error: %v", err)
			}
		})
	}
}

func TestCredentialReadRejectsUnownedPathsAndSymlinks(t *testing.T) {
	home := t.TempDir()
	for _, id := range []string{"", "../auth", "existing-app-account", "tui-codex-../../secret"} {
		if _, err := ReadCredential(home, Account{CredentialID: id}); !errors.Is(err, ErrCredentialUnavailable) {
			t.Fatal("accepted unowned credential path")
		}
	}
	id := "tui-codex-0123456789abcdef0123456789abcdef"
	path := filepath.Join(platform.RunwayDir(home), "accounts", id, "auth.json")
	setupFixtureFile(t, path, []byte(`{"apiKey":"fixture"}`))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "outside.json")
	if err := os.WriteFile(outside, []byte(`{"apiKey":"outside"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := ReadCredential(home, Account{CredentialID: id}); !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatal("followed credential symlink")
	}
}

func TestConfigureCancelHasNoWritesAndSecretInputIsHidden(t *testing.T) {
	for _, confirm := range []string{"0\n", "1\n"} {
		t.Run(confirm[:1], func(t *testing.T) {
			home := t.TempDir()
			var output bytes.Buffer
			term := &fakeTerminal{keys: [][]byte{
				[]byte("1\n300\n1\n2\n2\nhttps://example.invalid\nfixture-hidden-key\nsecret-paste-tail"),
				[]byte("more-secret-tail"), nil, []byte(confirm),
			}, readError: io.EOF}
			cfg, err := configure(context.Background(), &output, term, home, Config{})
			if strings.Contains(output.String(), "fixture-hidden-key") || strings.Contains(output.String(), "secret-paste-tail") || strings.Contains(output.String(), "more-secret-tail") {
				t.Fatal("secret input echoed")
			}
			if confirm == "0\n" {
				if !errors.Is(err, ErrSetupCanceled) {
					t.Fatalf("cancel error: %v", err)
				}
				if _, err := os.Stat(platform.RunwayDir(home)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("canceled setup wrote files")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				data, err := ReadCredential(home, cfg.Accounts[store.ProviderCodex])
				if err != nil || !bytes.Contains(data, []byte("fixture-hidden-key")) {
					t.Fatal("confirmed key missing")
				}
			}
		})
	}
}

func TestConfigureRejectsRedirectedStreamsBeforeCredentialAccess(t *testing.T) {
	in, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	defer write.Close()
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	home := t.TempDir()
	if _, err := Configure(context.Background(), in, out, home, Config{}); !errors.Is(err, ErrNotTerminal) {
		t.Fatal("redirected setup accepted")
	}
	if _, err := os.Stat(platform.RunwayDir(home)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("nonterminal setup touched account files")
	}
}

func TestConfigMissingIsDistinctFromMalformed(t *testing.T) {
	home := t.TempDir()
	if _, err := LoadConfig(home); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing error: %v", err)
	}
	cfg := setupTestConfig()
	data, _ := json.Marshal(cfg)
	setupFixtureFile(t, ConfigPath(home), data)
	if _, err := LoadConfig(home); err != nil {
		t.Fatal(err)
	}
}
