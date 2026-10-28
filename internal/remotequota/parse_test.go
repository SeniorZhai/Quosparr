package remotequota

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseKimiPoolsAndPlan(t *testing.T) {
	body := []byte(`{
		"usages": {
			"limit_5h": {"used_ratio": 0.25, "reset_time": "2026-10-28T08:00:00Z"},
			"limit_7d": {"used_ratio": 0.5, "reset_time": "2026-11-01T00:00:00Z"},
			"limit_month_total": {"used_ratio": 0.1, "reset_time": "2026-11-01T00:00:00Z"},
			"limit_month_code": {"used_ratio": 0.4}
		},
		"user": {"membership": {"level": "LEVEL_BASIC"}},
		"version": "GOODS_VERSION_V1"
	}`)
	got, err := ParseKimi(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "" || got.Plan != "Moderato" || len(got.Pools) != 3 {
		t.Fatalf("reading = %+v", got)
	}
	if got.Pools[0].ID != PoolSession || got.Pools[0].UsedPercent != 25 || got.Pools[0].WindowMinutes != 300 {
		t.Fatalf("session = %+v", got.Pools[0])
	}
	if got.Pools[0].ResetsAt != time.Date(2026, 10, 28, 8, 0, 0, 0, time.UTC) {
		t.Fatalf("reset = %s", got.Pools[0].ResetsAt)
	}
	if len(got.Lines) != 1 || got.Lines[0].ID != LineMonthCode || got.Lines[0].Value != "40%" {
		t.Fatalf("lines = %+v", got.Lines)
	}
}

func TestParseKimiKeepsUnknownPlanWhenVersionChanges(t *testing.T) {
	got, err := ParseKimi([]byte(`{"usages":{"limit_5h":{"used_ratio":0}},"user":{"membership":{"level":"LEVEL_BASIC"}},"version":"GOODS_VERSION_V2"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Plan != "LEVEL_BASIC" {
		t.Fatalf("plan = %q", got.Plan)
	}
}

func TestParseKimiRejectsEmptyUsage(t *testing.T) {
	got, err := ParseKimi([]byte(`{"usages":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != CodeFailed {
		t.Fatalf("code = %q", got.Code)
	}
}

func TestParseMoonshotBalance(t *testing.T) {
	got, err := ParseMoonshot([]byte(`{"code":0,"status":true,"data":{"available_balance":12.5,"voucher_balance":1,"cash_balance":11.5}}`), RegionChina)
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "" || len(got.Pools) != 1 || !got.Pools[0].Balance || got.Pools[0].ValueText != "¥12.50" || len(got.Lines) != 0 {
		t.Fatalf("reading = %+v", got)
	}
	intl, err := ParseMoonshot([]byte(`{"status":true,"data":{"available_balance":3,"cash_balance":-1.2}}`), RegionIntl)
	if err != nil {
		t.Fatal(err)
	}
	if intl.Pools[0].ValueText != "$3.00" || len(intl.Lines) != 1 || intl.Lines[0].ID != LineDeficit || intl.Lines[0].Value != "$-1.20" {
		t.Fatalf("intl = %+v", intl)
	}
	failed, err := ParseMoonshot([]byte(`{"code":1,"status":false}`), RegionChina)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Code != CodeFailed {
		t.Fatalf("code = %q", failed.Code)
	}
}

func TestParseDeepSeekBalance(t *testing.T) {
	got, err := ParseDeepSeek([]byte(`{
		"is_available": true,
		"balance_infos": [{"currency":"CNY","total_balance":"12.5","granted_balance":"2","topped_up_balance":"10.5"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "" || len(got.Pools) != 1 || got.Pools[0].ValueText != "¥12.50" || got.Pools[0].UsedPercent != 0 || !got.Pools[0].Balance {
		t.Fatalf("pool = %+v code %q", got.Pools, got.Code)
	}
	if len(got.Lines) != 2 || got.Lines[0].Value != "¥2.00" || got.Lines[1].Value != "¥10.50" {
		t.Fatalf("lines = %+v", got.Lines)
	}
}

func TestParseDeepSeekUnavailable(t *testing.T) {
	got, err := ParseDeepSeek([]byte(`{"is_available":false,"balance_infos":[{"currency":"USD","total_balance":"3","granted_balance":"0","topped_up_balance":"3"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != CodeUnavailable || got.Pools[0].UsedPercent != 100 || got.Pools[0].ValueText != "$3.00" {
		t.Fatalf("reading = %+v", got)
	}
	empty, err := ParseDeepSeek([]byte(`{"is_available":true,"balance_infos":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if empty.Code != CodeUnavailable || empty.Pools[0].ValueText != "0" {
		t.Fatalf("empty = %+v", empty)
	}
}

func TestParseCopilotSkipsUnlimitedAndPlaceholder(t *testing.T) {
	body := []byte(`{
		"copilot_plan": "individual",
		"quota_reset_date": "2026-11-01T00:00:00Z",
		"quota_snapshots": {
			"premium_interactions": {"percent_remaining": 40, "entitlement": 100, "remaining": 40, "unlimited": false},
			"chat": {"percent_remaining": 100, "entitlement": 0, "remaining": 0, "unlimited": false}
		}
	}`)
	got, err := ParseCopilot(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.Plan != "individual" || len(got.Pools) != 1 || got.Pools[0].ID != PoolPremium || got.Pools[0].UsedPercent != 60 {
		t.Fatalf("reading = %+v", got)
	}
	if got.Pools[0].ResetsAt.IsZero() {
		t.Fatal("missing reset")
	}
	unlimited, err := ParseCopilot([]byte(`{"copilot_plan":"business","token_based_billing":true,"quota_snapshots":{"premium_interactions":{"unlimited":true}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if unlimited.Code != "" || len(unlimited.Pools) != 0 || unlimited.Plan != "business" {
		t.Fatalf("unlimited = %+v", unlimited)
	}
	credits, err := ParseCopilot([]byte(`{"copilot_plan":"business","token_based_billing":true,"quota_snapshots":{"premium_interactions":{"unlimited":true,"credits_used":100},"chat":{"unlimited":true,"credits_used":4}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if credits.Code != "" || credits.Plan != "business" || len(credits.Pools) != 1 || credits.Pools[0].ID != PoolCredits || credits.Pools[0].ValueText != "100" {
		t.Fatalf("credits = %+v", credits)
	}
	failed, err := ParseCopilot([]byte(`{"copilot_plan":"individual","quota_snapshots":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	if failed.Code != CodeFailed {
		t.Fatalf("code = %q", failed.Code)
	}
}

func TestSaveLoadKeys(t *testing.T) {
	dir := t.TempDir()
	in := Keys{KimiKey: "kimi-key", KimiRegion: RegionIntl, DeepSeekKey: "ds-key", CopilotToken: "gh-token"}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Fatalf("keys = %+v", got)
	}
	info, err := pathInfo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info != 0o600 {
		t.Fatalf("mode = %o", info)
	}
	if err := Save(dir, Keys{KimiKey: "has space"}); err == nil || !strings.Contains(err.Error(), "kimi") {
		t.Fatalf("err = %v", err)
	}
}

func pathInfo(dir string) (int, error) {
	st, err := os.Stat(Path(dir))
	if err != nil {
		return 0, err
	}
	return int(st.Mode().Perm()), nil
}
