package tui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/licoy/quosparr/internal/apiquota"
	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/remotequota"
	"github.com/licoy/quosparr/internal/store"
)

// importAppCredential reads only the selected App account and strips unrelated fields.
func importAppCredential(home string, provider store.Provider) (Account, []byte, error) {
	account := Account{Mode: "imported"}
	root := platform.RunwayDir(home)
	var data []byte
	var err error
	if provider.RemoteQuota() {
		data, err = readConfigFile(filepath.Join(root, remotequota.FileName), false)
		if err != nil {
			return Account{}, nil, ErrCredentialUnavailable
		}
		var keys remotequota.Keys
		if json.Unmarshal(data, &keys) != nil {
			return Account{}, nil, ErrCredentialUnavailable
		}
		var key string
		switch provider {
		case store.ProviderKimi:
			key, account.Region = keys.KimiKey, keys.KimiRegion
			if account.Region == "" {
				account.Region = "cn"
			}
		case store.ProviderDeepSeek:
			key = keys.DeepSeekKey
		case store.ProviderCopilot:
			key = keys.CopilotToken
		}
		data, err = apiCredential(key)
		if err != nil {
			return Account{}, nil, err
		}
		return account, data, nil
	}
	index := "index.json"
	if provider == store.ProviderClaude {
		index = "claude-index.json"
	}
	if provider == store.ProviderGrok {
		index = "grok-index.json"
	}
	data, err = readConfigFile(filepath.Join(root, "accounts", index), false)
	if err != nil {
		return Account{}, nil, ErrCredentialUnavailable
	}
	var idx struct {
		Active   string `json:"activeAccountId"`
		Current  string `json:"currentAccountID"`
		Accounts []struct {
			ID         string `json:"id"`
			APIBaseURL string `json:"apiBaseURL"`
			API        *struct {
				BaseURL string `json:"baseURL"`
			} `json:"api"`
		} `json:"accounts"`
	}
	if json.Unmarshal(data, &idx) != nil {
		return Account{}, nil, ErrCredentialUnavailable
	}
	id := idx.Current
	if provider == store.ProviderCodex {
		id = idx.Active
	}
	if id == "" || len(id) > 200 || id == "." || id == ".." || strings.IndexFunc(id, func(r rune) bool {
		return r != '-' && r != '_' && r != '.' && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9')
	}) >= 0 {
		return Account{}, nil, ErrCredentialUnavailable
	}
	found := false
	for _, row := range idx.Accounts {
		if row.ID != id {
			continue
		}
		found = true
		account.BaseURL = row.APIBaseURL
		if row.API != nil {
			account.BaseURL = row.API.BaseURL
		}
		break
	}
	if !found {
		return Account{}, nil, ErrCredentialUnavailable
	}
	if account.BaseURL != "" {
		urls, err := apiquota.NormalizeBaseURL(account.BaseURL)
		if err != nil {
			return Account{}, nil, ErrCredentialUnavailable
		}
		account.BaseURL = urls.OpenAIBase
		if provider == store.ProviderClaude {
			account.BaseURL = urls.AnthropicBase
		}
	}
	data, err = readConfigFile(filepath.Join(root, "accounts", id, "auth.json"), false)
	if err != nil {
		return Account{}, nil, ErrCredentialUnavailable
	}
	switch provider {
	case store.ProviderCodex:
		auth, err := codex.DecodeAuth(data)
		if err != nil {
			return Account{}, nil, ErrCredentialUnavailable
		}
		if auth.IsAPIKey() {
			data, err = apiCredential(auth.APIKey())
			if err != nil {
				return Account{}, nil, err
			}
		} else {
			if auth.Tokens.AccessToken == "" {
				return Account{}, nil, ErrCredentialUnavailable
			}
			data, err = json.Marshal(map[string]any{"auth_mode": auth.AuthMode, "tokens": map[string]string{"access_token": auth.Tokens.AccessToken, "id_token": auth.Tokens.IDToken, "account_id": auth.Tokens.AccountID}})
			if err != nil {
				return Account{}, nil, ErrCredentialUnavailable
			}
		}
	case store.ProviderClaude:
		var key struct {
			APIKey string `json:"apiKey"`
		}
		if json.Unmarshal(data, &key) == nil && key.APIKey != "" {
			data, err = apiCredential(key.APIKey)
			if err != nil {
				return Account{}, nil, err
			}
		} else {
			doc, err := claude.ParseAuthDocument(data)
			if err != nil || doc.OAuth().AccessToken == "" {
				return Account{}, nil, ErrCredentialUnavailable
			}
			oauth := doc.OAuth()
			fields := map[string]any{"accessToken": oauth.AccessToken, "scopes": oauth.Scopes, "subscriptionType": oauth.SubscriptionType, "rateLimitTier": oauth.RateLimitTier}
			if oauth.HasExpiry {
				fields["expiresAt"] = oauth.ExpiresAt.UnixMilli()
			}
			data, err = json.Marshal(map[string]any{"claudeAiOauth": fields})
			if err != nil {
				return Account{}, nil, ErrCredentialUnavailable
			}
		}
	case store.ProviderGrok:
		doc, err := grok.ParseAuthDocument(data)
		if err != nil || !doc.Identity.HasAccessToken {
			return Account{}, nil, ErrCredentialUnavailable
		}
		var scopes map[string]json.RawMessage
		if json.Unmarshal(data, &scopes) != nil {
			return Account{}, nil, ErrCredentialUnavailable
		}
		var scope map[string]json.RawMessage
		if json.Unmarshal(scopes[doc.Identity.Scope], &scope) != nil {
			return Account{}, nil, ErrCredentialUnavailable
		}
		fields := map[string]json.RawMessage{}
		for _, key := range []string{"key", "user_id", "principal_id", "principal_type", "email", "team_id", "team_name", "oidc_issuer", "oidc_client_id", "expires_at"} {
			if value, ok := scope[key]; ok {
				fields[key] = value
			}
		}
		data, err = json.Marshal(map[string]any{doc.Identity.Scope: fields})
		if err != nil {
			return Account{}, nil, ErrCredentialUnavailable
		}
	default:
		return Account{}, nil, ErrCredentialUnavailable
	}
	return account, data, nil
}

func apiCredential(key string) ([]byte, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 8192 || strings.IndexFunc(key, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 {
		return nil, ErrCredentialUnavailable
	}
	return json.Marshal(struct {
		APIKey string `json:"apiKey"`
	}{key})
}
