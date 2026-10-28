package apiquota

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"
)

const upstreamQuotaPath = "/v1/magpie/quotas"

func (p prober) upstream(ctx context.Context, base, provider string) (Quota, bool) {
	origin, ok := siteOrigin(base)
	if !ok {
		return Quota{}, false
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/", nil)
	req.Header.Set("Accept", "application/json")
	status, body, err := doRequest(p.client, req, maxBodyBytes)
	var metadata struct {
		APIs []string `json:"apis"`
	}
	if err != nil || status != http.StatusOK || json.Unmarshal(body, &metadata) != nil || !slices.Contains(metadata.APIs, upstreamQuotaPath) {
		return Quota{}, false
	}
	endpoint := origin + upstreamQuotaPath
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.key)
	status, body, err = doRequest(p.client, req, maxBodyBytes)
	if err != nil {
		return quiet(StatusError, requestMessage(err, p.key), endpoint, p.now), true
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return quiet(StatusUnauthorized, "", endpoint, p.now), true
	}
	if status != http.StatusOK {
		return quiet(StatusUnsupported, "", endpoint, p.now), true
	}
	q := parseUpstream(body, provider, p.now)
	q.Endpoint = endpoint
	return q, true
}

func parseUpstream(body []byte, provider string, now time.Time) Quota {
	var document struct {
		Data []struct {
			Kind     string          `json:"kind"`
			Provider string          `json:"provider"`
			ReadAt   time.Time       `json:"readAt"`
			Windows  json.RawMessage `json:"windows"`
		} `json:"data"`
	}
	invalid := quiet(StatusInvalidData, msgInvalidQuota, "", now)
	if json.Unmarshal(body, &document) != nil {
		return invalid
	}
	selected := -1
	for i, row := range document.Data {
		family := strings.FieldsFunc(strings.ToLower(row.Provider), func(r rune) bool {
			return r == '-' || r == '_' || r == ':' || r == '/' || r == '.' || unicode.IsSpace(r)
		})
		if row.Kind != "subscription" || !slices.Contains(family, provider) {
			continue
		}
		if selected >= 0 {
			return quiet(StatusUnsupported, "multiple upstream accounts match provider", "", now)
		}
		selected = i
	}
	if selected < 0 {
		return quiet(StatusUnsupported, "", "", now)
	}
	row := document.Data[selected]
	var windows []struct {
		Name      string     `json:"name"`
		Used      *float64   `json:"used"`
		Remaining *float64   `json:"remaining"`
		ResetsAt  *time.Time `json:"resetsAt"`
	}
	if json.Unmarshal(row.Windows, &windows) != nil {
		return invalid
	}
	q := Quota{Status: StatusAvailable, Source: SourceUpstream, Provider: provider, FetchedAt: row.ReadAt}
	if q.FetchedAt.IsZero() {
		q.FetchedAt = now
	}
	for _, w := range windows {
		if w.Used == nil || w.Remaining == nil || !finite(*w.Used) || !finite(*w.Remaining) || *w.Used < 0 || *w.Used > 100 || *w.Remaining < 0 || *w.Remaining > 100 || math.Abs(*w.Used+*w.Remaining-100) > 0.01 {
			return invalid
		}
		title := strings.Map(func(r rune) rune {
			if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
				return -1
			}
			return r
		}, w.Name)
		title = strings.TrimSpace(title)
		if title == "" || len([]rune(title)) > 80 {
			return invalid
		}
		window := Window{Title: title, UsedPercent: *w.Used, RemainingPercent: *w.Remaining}
		lower := strings.ToLower(title)
		switch {
		case strings.Contains(lower, "5") && (strings.Contains(lower, "h") || strings.Contains(lower, "小时")):
			window.WindowMinutes = 300
		case strings.Contains(lower, "week") || strings.Contains(lower, "周") || strings.Contains(lower, "7") && (strings.Contains(lower, "day") || strings.Contains(lower, "d") || strings.Contains(lower, "天")):
			window.WindowMinutes = 10080
		}
		if w.ResetsAt != nil {
			window.ResetsAt = *w.ResetsAt
		}
		q.Windows = append(q.Windows, window)
	}
	if len(q.Windows) == 0 {
		q.Status = StatusUnsupported
	}
	return q
}
