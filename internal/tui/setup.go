package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/licoy/quosparr/internal/apiquota"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

// Configure keeps edits and imported credentials in memory until Save is confirmed.
func Configure(ctx context.Context, in, out *os.File, home string, current Config) (cfg Config, err error) {
	if err := CheckTerminal(in, out); err != nil {
		return Config{}, err
	}
	term, err := openTerminal(in, out)
	if err != nil {
		return Config{}, err
	}
	defer func() { err = errors.Join(err, term.restore()) }()
	return configure(ctx, out, term, home, current)
}

func configure(ctx context.Context, out io.Writer, term terminal, home string, current Config) (Config, error) {
	cfg := current
	cfg.Providers = append([]store.Provider(nil), current.Providers...)
	cfg.Accounts = make(map[store.Provider]Account, len(current.Accounts))
	for p, account := range current.Accounts {
		cfg.Accounts[p] = account
	}
	if len(cfg.Providers) == 0 {
		cfg.Providers = []store.Provider{store.ProviderCodex, store.ProviderClaude, store.ProviderGrok}
		for _, p := range cfg.Providers {
			cfg.Accounts[p] = Account{Mode: "cli"}
		}
	}
	if cfg.IntervalSeconds == 0 {
		cfg.IntervalSeconds = 300
	}
	if !cfg.Language.Valid() {
		cfg.Language = l10n.Resolve("system", l10n.SystemPreferredLanguages())
	}
	l := l10n.New(cfg.Language)
	input := setupInput{ctx: ctx, term: term, out: out}
	if err := input.message("\r\nQuosparr · " + l.TUI("setup") + "\r\n" + l.TUI("setupIntro") + "\r\n"); err != nil {
		return Config{}, err
	}
	var choices, selected []string
	for i, p := range setupProviders {
		choices = append(choices, fmt.Sprintf("%d %s", i+1, setupProviderName(p)))
		for _, current := range cfg.Providers {
			if current == p {
				selected = append(selected, strconv.Itoa(i+1))
			}
		}
	}
	if err := input.message(strings.Join(choices, " · ") + "\r\n"); err != nil {
		return Config{}, err
	}
	for {
		value, err := input.field(l.TUI("providers"), strings.Join(selected, ","), false)
		if err != nil {
			return Config{}, err
		}
		var providers []store.Provider
		valid := true
		seen := map[store.Provider]bool{}
		for _, number := range strings.Fields(strings.ReplaceAll(value, ",", " ")) {
			n, err := strconv.Atoi(number)
			if err != nil || n < 1 || n > len(setupProviders) || seen[setupProviders[n-1]] {
				valid = false
				break
			}
			p := setupProviders[n-1]
			seen[p] = true
			providers = append(providers, p)
		}
		if valid && len(providers) != 0 {
			cfg.Providers = providers
			break
		}
		if err := input.message(l.TUI("invalidChoice") + "\r\n"); err != nil {
			return Config{}, err
		}
	}
	for {
		value, err := input.field(l.TUI("intervalSeconds"), strconv.Itoa(cfg.IntervalSeconds), false)
		if err != nil {
			return Config{}, err
		}
		n, err := strconv.Atoi(value)
		if err == nil && n >= 30 && n <= 86400 {
			cfg.IntervalSeconds = n
			break
		}
		if err := input.message(l.TUI("invalidChoice") + "\r\n"); err != nil {
			return Config{}, err
		}
	}
	choices = nil
	languageDefault := "1"
	for i, lang := range l10n.Languages() {
		choices = append(choices, fmt.Sprintf("%d %s", i+1, lang.NativeTitle()))
		if cfg.Language == lang {
			languageDefault = strconv.Itoa(i + 1)
		}
	}
	if err := input.message(strings.Join(choices, " · ") + "\r\n"); err != nil {
		return Config{}, err
	}
	for {
		value, err := input.field(l.TUI("language"), languageDefault, false)
		if err != nil {
			return Config{}, err
		}
		n, err := strconv.Atoi(value)
		if err == nil && n >= 1 && n <= len(l10n.Languages()) {
			cfg.Language = l10n.Languages()[n-1]
			l = l10n.New(cfg.Language)
			break
		}
		if err := input.message(l.TUI("invalidChoice") + "\r\n"); err != nil {
			return Config{}, err
		}
	}
	pending := map[store.Provider][]byte{}
	var source string
	for {
		value, err := input.field(l.TUI("sourceChoices"), "1", false)
		if err != nil {
			return Config{}, err
		}
		if value == "1" || value == "2" || value == "3" {
			source = value
			break
		}
		if err := input.message(l.TUI("invalidChoice") + "\r\n"); err != nil {
			return Config{}, err
		}
	}
	for _, p := range cfg.Providers {
		if source == "3" {
			account, data, err := importAppCredential(home, p)
			status := l.TUI("importMissing")
			if err == nil {
				cfg.Accounts[p], pending[p] = account, data
				status = l.TUI("importReady")
			}
			if err := input.message(setupProviderName(p) + ": " + status + "\r\n"); err != nil {
				return Config{}, err
			}
			continue
		}
		if source == "1" {
			if _, exists := cfg.Accounts[p]; !exists && p != store.ProviderKimi && p != store.ProviderDeepSeek {
				cfg.Accounts[p] = Account{Mode: "cli"}
			}
			continue
		}
		if err := input.message("\r\n" + setupProviderName(p) + "\r\n"); err != nil {
			return Config{}, err
		}
		choiceDefault := "3"
		if _, exists := cfg.Accounts[p]; !exists {
			choiceDefault = "2"
		}
		var mode string
		accountChoices := l.TUI("accountChoices")
		if p == store.ProviderKimi || p == store.ProviderDeepSeek {
			accountChoices = l.TUI("keyChoices")
		}
		for {
			value, err := input.field(accountChoices, choiceDefault, false)
			if err != nil {
				return Config{}, err
			}
			if value == "3" || value == "2" || value == "1" && p != store.ProviderKimi && p != store.ProviderDeepSeek {
				mode = value
				break
			}
			if err := input.message(l.TUI("invalidChoice") + "\r\n"); err != nil {
				return Config{}, err
			}
		}
		if mode == "3" {
			continue
		}
		if mode == "1" {
			cfg.Accounts[p] = Account{Mode: "cli"}
			continue
		}
		account := Account{Mode: "api", Region: cfg.Accounts[p].Region}
		if !p.RemoteQuota() {
			baseDefault := cfg.Accounts[p].BaseURL
			if baseDefault == "" {
				switch p {
				case store.ProviderCodex:
					baseDefault = "https://api.openai.com/v1"
				case store.ProviderClaude:
					baseDefault = "https://api.anthropic.com"
				case store.ProviderGrok:
					baseDefault = "https://api.x.ai/v1"
				}
			}
			for {
				value, err := input.field("Base URL", baseDefault, false)
				if err != nil {
					return Config{}, err
				}
				urls, err := apiquota.NormalizeBaseURL(value)
				if err == nil {
					account.BaseURL = urls.OpenAIBase
					if p == store.ProviderClaude {
						account.BaseURL = urls.AnthropicBase
					}
					break
				}
				if err := input.message(l.TUI("invalidURL") + "\r\n"); err != nil {
					return Config{}, err
				}
			}
		}
		if p == store.ProviderKimi {
			if account.Region == "" {
				account.Region = "cn"
			}
			for {
				value, err := input.field(l.TUI("kimiRegion"), account.Region, false)
				if err != nil {
					return Config{}, err
				}
				if value == "cn" || value == "intl" {
					account.Region = value
					break
				}
				if err := input.message(l.TUI("invalidChoice") + "\r\n"); err != nil {
					return Config{}, err
				}
			}
		}
		for {
			value, err := input.field(l.TUI("apiKeyHidden"), "", true)
			if err != nil {
				return Config{}, err
			}
			data, err := apiCredential(value)
			if err == nil {
				pending[p], cfg.Accounts[p] = data, account
				break
			}
			if err := input.message(l.TUI("invalidKey") + "\r\n"); err != nil {
				return Config{}, err
			}
		}
	}
	var summary []string
	for _, p := range cfg.Providers {
		mode := cfg.Accounts[p].Mode
		label := l.TUI("unconfigured")
		switch mode {
		case "cli":
			label = l.TUI("cliSource")
		case "api":
			label = "API Key"
		case "imported":
			label = l.TUI("importSource")
		}
		summary = append(summary, setupProviderName(p)+": "+label)
	}
	if err := input.message("\r\n" + strings.Join(summary, "\r\n") + "\r\n" + l.TUI("saveNotice") + "\r\n"); err != nil {
		return Config{}, err
	}
	confirm, err := input.field(l.TUI("savePrompt"), "0", false)
	if err != nil {
		return Config{}, err
	}
	if confirm != "1" {
		return Config{}, ErrSetupCanceled
	}
	if ctx.Err() != nil {
		return Config{}, ErrSetupCanceled
	}
	return saveConfig(home, cfg, pending)
}

type setupInput struct {
	ctx     context.Context
	term    terminal
	out     io.Writer
	pending []byte
}

func (s *setupInput) message(text string) error { _, err := io.WriteString(s.out, text); return err }

// field never echoes hidden input, including pasted keys and validation failures.
func (s *setupInput) field(label, defaultValue string, hidden bool) (string, error) {
	prompt := label
	if defaultValue != "" {
		prompt += " [" + defaultValue + "]"
	}
	prompt += ": "
	if err := s.message(prompt); err != nil {
		return "", err
	}
	var value []byte
	for s.ctx.Err() == nil {
		if len(s.pending) == 0 {
			data, err := s.term.read()
			if err != nil {
				return "", err
			}
			s.pending = data
			if len(s.pending) == 0 {
				continue
			}
		}
		key := s.pending[0]
		s.pending = s.pending[1:]
		switch key {
		case 3, 4, 27:
			return "", ErrSetupCanceled
		case '\r', '\n':
			if key == '\r' && len(s.pending) > 0 && s.pending[0] == '\n' {
				s.pending = s.pending[1:]
			}
			if hidden {
				// Discard the rest of a multiline paste before any visible prompt.
				s.pending = nil
				for {
					extra, err := s.term.read()
					if err != nil {
						return "", err
					}
					if len(extra) == 0 {
						break
					}
					for _, key := range extra {
						if key == 3 || key == 27 {
							return "", ErrSetupCanceled
						}
					}
				}
			}
			if err := s.message("\r\n"); err != nil {
				return "", err
			}
			if len(value) == 0 {
				return defaultValue, nil
			}
			return string(value), nil
		case 8, 127:
			if len(value) > 0 {
				_, n := utf8.DecodeLastRune(value)
				value = value[:len(value)-n]
			}
		default:
			if key < 32 {
				continue
			}
			if len(value) >= 8192 {
				return "", ErrConfigInvalid
			}
			value = append(value, key)
		}
		if !hidden {
			w, _, err := s.term.size()
			if err != nil {
				return "", err
			}
			if err := s.message("\r\x1b[2K" + fit(prompt+string(value), max(1, w-1))); err != nil {
				return "", err
			}
		}
	}
	return "", ErrSetupCanceled
}

func setupProviderName(p store.Provider) string {
	switch p {
	case store.ProviderCodex:
		return "Codex"
	case store.ProviderClaude:
		return "Claude"
	case store.ProviderGrok:
		return "Grok"
	case store.ProviderKimi:
		return "Kimi Code"
	case store.ProviderDeepSeek:
		return "DeepSeek"
	case store.ProviderCopilot:
		return "GitHub Copilot"
	}
	return ""
}
