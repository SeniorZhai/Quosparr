package remotequota

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/licoy/quosparr/internal/netx"
)

const maxBody = 1 << 20

// FetchKimi reads Kimi Code usage for an API key.
func FetchKimi(ctx context.Context, key, region string) (Reading, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Reading{Code: CodeUnconfigured}, nil
	}
	host := "https://api.kimi.com"
	if normalizeRegion(region) == RegionIntl {
		host = "https://api.kimi.ai"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/coding/v1/usages", nil)
	if err != nil {
		return Reading{Code: CodeFailed}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	reading, err := do(ctx, req, ParseKimi)
	if reading.Code != CodeUnauthorized {
		return reading, err
	}
	return fetchMoonshot(ctx, key, region)
}

// fetchMoonshot reads an Open Platform balance. Kimi Code and that platform
// use different keys; a key rejected by Kimi Code is tried here.
func fetchMoonshot(ctx context.Context, key, region string) (Reading, error) {
	host := "https://api.moonshot.cn"
	if normalizeRegion(region) == RegionIntl {
		host = "https://api.moonshot.ai"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/v1/users/me/balance", nil)
	if err != nil {
		return Reading{Code: CodeFailed}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	return do(ctx, req, func(body []byte) (Reading, error) {
		return ParseMoonshot(body, region)
	})
}

// FetchDeepSeek reads the DeepSeek API balance.
func FetchDeepSeek(ctx context.Context, key string) (Reading, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Reading{Code: CodeUnconfigured}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.deepseek.com/user/balance", nil)
	if err != nil {
		return Reading{Code: CodeFailed}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	return do(ctx, req, ParseDeepSeek)
}

// FetchCopilot reads GitHub Copilot quota. An empty token tries `gh auth token`.
func FetchCopilot(ctx context.Context, token string) (Reading, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		var err error
		token, err = GitHubToken(ctx)
		if err != nil || token == "" {
			return Reading{Code: CodeUnconfigured}, nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/copilot_internal/user", nil)
	if err != nil {
		return Reading{Code: CodeFailed}, err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Editor-Version", "vscode/1.96.2")
	req.Header.Set("Editor-Plugin-Version", "copilot-chat/0.26.7")
	req.Header.Set("User-Agent", "GitHubCopilotChat/0.26.7")
	req.Header.Set("X-Github-Api-Version", "2025-04-01")
	return do(ctx, req, ParseCopilot)
}

func do(ctx context.Context, req *http.Request, parse func([]byte) (Reading, error)) (Reading, error) {
	client, err := netx.ClientFor(ctx, netx.WithoutCookies)
	if err != nil {
		return Reading{Code: CodeFailed}, err
	}
	res, err := client.Do(req)
	if err != nil {
		return Reading{Code: CodeFailed}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return Reading{Code: CodeFailed}, err
	}
	switch res.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return Reading{Code: CodeUnauthorized}, nil
	case http.StatusOK:
		return parse(body)
	default:
		return Reading{Code: CodeFailed}, nil
	}
}
