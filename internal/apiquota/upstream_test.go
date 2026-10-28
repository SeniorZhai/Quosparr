package apiquota

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const upstreamFixture = `{"data":[
 {"kind":"subscription","provider":"codex","name":"private account","user":"private user","windows":[{"name":"7 day","used":90,"remaining":10}]},
 {"kind":"subscription","provider":"gateway-claude","name":"private account","user":"private user","readAt":"2026-08-21T12:00:00Z","windows":[
  {"name":"5 hour","used":42,"remaining":58,"resetsAt":"2026-08-21T16:00:00Z"},
  {"name":"7 day","used":21,"remaining":79,"resetsAt":"2026-08-25T12:00:00Z"},
  {"name":"7 day (special)","used":2,"remaining":98,"resetsAt":"2026-08-25T12:00:00Z"}]}
]}`

func TestProbeAdvertisedUpstreamQuotaForOneProvider(t *testing.T) {
	site := newFakeSite(t, map[string]reply{
		"/":               {200, `{"apis":["/v1/magpie/quotas"]}`},
		upstreamQuotaPath: {200, upstreamFixture},
	})
	q := ProbeForProvider(context.Background(), site.client(), site.url(), "fixture-key", "claude", testNow)
	if q.Status != StatusAvailable || q.Source != SourceUpstream || q.Provider != "claude" || len(q.Windows) != 3 || q.Remaining != nil || q.Unit != "" {
		t.Fatalf("upstream quota not separated from balance: %+v", q)
	}
	if q.Windows[0].RemainingPercent != 58 || q.Windows[0].WindowMinutes != 300 || q.Windows[1].RemainingPercent != 79 || q.Windows[1].WindowMinutes != 10080 {
		t.Fatalf("wrong provider/window: %+v", q.Windows)
	}
	if root, _ := site.find("/"); root.auth != "" {
		t.Fatal("public discovery received a credential")
	}
	if quota, ok := site.find(upstreamQuotaPath); !ok || quota.auth != "Bearer fixture-key" {
		t.Fatal("advertised quota did not receive its credential")
	}
	encoded, _ := json.Marshal(q)
	for _, unwanted := range []string{"private account", "private user", "fixture-key"} {
		if strings.Contains(string(encoded), unwanted) {
			t.Fatal("upstream identity escaped into cached quota")
		}
	}
}

func TestUpstreamRequiresAdvertisedEndpointAndUnambiguousPercentages(t *testing.T) {
	for _, root := range []string{`{}`, `{"apis":["https://other.example/v1/magpie/quotas"]}`, `{"apis":["/v1/magpie/quotas?token=secret"]}`} {
		site := newFakeSite(t, map[string]reply{"/": {200, root}, upstreamQuotaPath: {200, upstreamFixture}})
		q := ProbeForProvider(context.Background(), site.client(), site.url(), "fixture-key", "claude", testNow)
		if len(q.Windows) != 0 {
			t.Fatal("unadvertised upstream quota accepted")
		}
		if _, ok := site.find(upstreamQuotaPath); ok {
			t.Fatal("unadvertised credential request")
		}
	}
	for _, tc := range []struct {
		name, body string
		status     Status
	}{
		{"wrong provider", strings.ReplaceAll(upstreamFixture, "gateway-claude", "unrelated"), StatusUnsupported},
		{"ambiguous", strings.ReplaceAll(upstreamFixture, `"provider":"codex"`, `"provider":"claude"`), StatusUnsupported},
		{"inconsistent percentages", strings.ReplaceAll(upstreamFixture, `"remaining":58`, `"remaining":60`), StatusInvalidData},
		{"missing percentage", strings.ReplaceAll(upstreamFixture, `"used":42,`, ``), StatusInvalidData},
		{"invalid reset", strings.ReplaceAll(upstreamFixture, "2026-08-21T16:00:00Z", "invalid"), StatusInvalidData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := parseUpstream([]byte(tc.body), "claude", testNow)
			if q.Status != tc.status || len(q.Windows) != 0 {
				t.Fatalf("unexpected result: %+v", q)
			}
		})
	}
}

func TestUpstreamWindowTextCannotEchoCredential(t *testing.T) {
	site := newFakeSite(t, map[string]reply{"/": {200, `{"apis":["/v1/magpie/quotas"]}`}, upstreamQuotaPath: {200, strings.ReplaceAll(upstreamFixture, "5 hour", "5 hour fixture-key")}})
	q := ProbeForProvider(context.Background(), site.client(), site.url(), "fixture-key", "claude", testNow)
	encoded, _ := json.Marshal(q)
	if strings.Contains(string(encoded), "fixture-key") {
		t.Fatal("window name echoed the API key")
	}
}
