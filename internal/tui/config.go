package tui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/licoy/quosparr/internal/apiquota"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/store"
)

var (
	ErrConfigInvalid         = errors.New("invalid TUI configuration")
	ErrConfigUnavailable     = errors.New("TUI configuration unavailable")
	ErrCredentialUnavailable = errors.New("TUI credential unavailable")
	ErrSetupCanceled         = errors.New("TUI setup canceled")
)

type Config struct {
	Providers       []store.Provider           `json:"providers"`
	IntervalSeconds int                        `json:"intervalSeconds"`
	Language        l10n.Language              `json:"language"`
	Accounts        map[store.Provider]Account `json:"accounts"`
}

type Account struct {
	Mode         string `json:"mode"`
	CredentialID string `json:"credentialID,omitempty"`
	BaseURL      string `json:"baseURL,omitempty"`
	Region       string `json:"region,omitempty"`
}

var setupProviders = []store.Provider{store.ProviderCodex, store.ProviderClaude, store.ProviderGrok, store.ProviderKimi, store.ProviderDeepSeek, store.ProviderCopilot}

func ConfigPath(home string) string {
	return filepath.Join(platform.RunwayDir(home), "tui", "config.json")
}

func LoadConfig(home string) (Config, error) {
	path := ConfigPath(home)
	data, err := readConfigFile(path, true)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, os.ErrNotExist
	}
	if err != nil {
		return Config{}, ErrConfigUnavailable
	}
	var cfg Config
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, ErrConfigInvalid
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Config{}, ErrConfigInvalid
	}
	if err := validateConfig(cfg, false); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// ReadCredential reads only a dedicated TUI credential, never an App account.
func ReadCredential(home string, account Account) ([]byte, error) {
	if !validCredentialID(account.CredentialID) {
		return nil, ErrCredentialUnavailable
	}
	root := platform.RunwayDir(home)
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrCredentialUnavailable
	}
	path := filepath.Join(root, "accounts", account.CredentialID, "auth.json")
	data, err := readConfigFile(path, true)
	if err != nil {
		return nil, ErrCredentialUnavailable
	}
	return data, nil
}

func validCredentialID(id string) bool {
	for _, prefix := range []string{"tui-codex-", "claude-tui-", "claude-api-tui-", "grok-tui-", "tui-kimi-", "tui-deepseek-", "tui-copilot-"} {
		if suffix, ok := strings.CutPrefix(id, prefix); ok && len(suffix) == 32 {
			_, err := hex.DecodeString(suffix)
			return err == nil
		}
	}
	return false
}

func validateConfig(cfg Config, pending bool) error {
	if len(cfg.Providers) == 0 || len(cfg.Providers) > len(setupProviders) || cfg.IntervalSeconds < 30 || cfg.IntervalSeconds > 86400 || !cfg.Language.Valid() {
		return ErrConfigInvalid
	}
	seen := map[store.Provider]bool{}
	for _, p := range cfg.Providers {
		if !setupProvider(p) || seen[p] {
			return ErrConfigInvalid
		}
		seen[p] = true
	}
	for p, account := range cfg.Accounts {
		if !setupProvider(p) {
			return ErrConfigInvalid
		}
		switch account.Mode {
		case "cli":
			if account.CredentialID != "" || account.BaseURL != "" || p == store.ProviderKimi || p == store.ProviderDeepSeek {
				return ErrConfigInvalid
			}
		case "api", "imported":
			if (!pending || account.CredentialID != "") && !validCredentialID(account.CredentialID) {
				return ErrConfigInvalid
			}
		default:
			return ErrConfigInvalid
		}
		if account.BaseURL != "" {
			u, err := apiquota.NormalizeBaseURL(account.BaseURL)
			if err != nil || (account.BaseURL != u.OpenAIBase && account.BaseURL != u.AnthropicBase) {
				return ErrConfigInvalid
			}
		}
		if account.Region != "" && (p != store.ProviderKimi || account.Region != "cn" && account.Region != "intl") {
			return ErrConfigInvalid
		}
	}
	return nil
}

func setupProvider(p store.Provider) bool {
	for _, known := range setupProviders {
		if p == known {
			return true
		}
	}
	return false
}

// readConfigFile rejects symlinks and non-regular files before reading bounded JSON.
func readConfigFile(path string, private bool) ([]byte, error) {
	for _, parent := range []string{filepath.Dir(filepath.Dir(path)), filepath.Dir(path)} {
		info, err := os.Lstat(parent)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrCredentialUnavailable
		}
		if private {
			if err := platform.VerifyOwnerOnly(parent, 0o700); err != nil {
				return nil, err
			}
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, ErrCredentialUnavailable
	}
	if private {
		if err := platform.VerifyOwnerOnly(path, 0o600); err != nil {
			return nil, err
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrCredentialUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, ErrCredentialUnavailable
	}
	return data, nil
}

func ensureConfigDirectory(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrConfigUnavailable
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrConfigUnavailable
	}
	if err := platform.EnsurePrivateDirectory(path); err != nil {
		return ErrConfigUnavailable
	}
	return nil
}

// saveConfig commits the config last; rollback only removes new credential copies.
func saveConfig(home string, cfg Config, pending map[store.Provider][]byte) (saved Config, err error) {
	if err := validateConfig(cfg, true); err != nil {
		return Config{}, err
	}
	root := platform.RunwayDir(home)
	for _, dir := range []string{root, filepath.Join(root, "tui"), filepath.Join(root, "accounts")} {
		if err := ensureConfigDirectory(dir); err != nil {
			return Config{}, err
		}
	}
	var created []string
	defer func() {
		if err == nil {
			return
		}
		for _, dir := range created {
			if removeErr := os.Remove(filepath.Join(dir, "auth.json")); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, errors.New("new TUI credential rollback failed"))
			}
			if removeErr := os.Remove(dir); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, errors.New("new TUI credential directory rollback failed"))
			}
		}
	}()
	saved = cfg
	saved.Accounts = make(map[store.Provider]Account, len(cfg.Accounts))
	for p, a := range cfg.Accounts {
		saved.Accounts[p] = a
	}
	for p, data := range pending {
		account, ok := saved.Accounts[p]
		if !ok || account.Mode == "cli" || len(data) == 0 || len(data) > 1<<20 || !json.Valid(data) {
			return Config{}, ErrConfigInvalid
		}
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return Config{}, ErrConfigUnavailable
		}
		prefix := "tui-" + string(p) + "-"
		switch p {
		case store.ProviderClaude:
			prefix = "claude-tui-"
			var key struct {
				APIKey string `json:"apiKey"`
			}
			if json.Unmarshal(data, &key) == nil && key.APIKey != "" {
				prefix = "claude-api-tui-"
			}
		case store.ProviderGrok:
			prefix = "grok-tui-"
		}
		account.CredentialID = prefix + hex.EncodeToString(random[:])
		dir := filepath.Join(root, "accounts", account.CredentialID)
		if err := os.Mkdir(dir, 0o700); err != nil {
			return Config{}, ErrConfigUnavailable
		}
		created = append(created, dir)
		if err := platform.ProtectOwnerOnly(dir, 0o700); err != nil {
			return Config{}, ErrConfigUnavailable
		}
		if err := platform.WriteFileAtomic(filepath.Join(dir, "auth.json"), data, 0o600); err != nil {
			return Config{}, ErrConfigUnavailable
		}
		saved.Accounts[p] = account
	}
	if err := validateConfig(saved, false); err != nil {
		return Config{}, err
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return Config{}, ErrConfigInvalid
	}
	if err := platform.WriteFileAtomic(ConfigPath(home), append(data, '\n'), 0o600); err != nil {
		return Config{}, ErrConfigUnavailable
	}
	return saved, nil
}
