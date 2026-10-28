package runway

import (
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/apiquota"
	"github.com/licoy/quosparr/internal/store"
)

func f64(v float64) *float64 { return &v }

func TestAPIQuotaWindowsAreSharedPercentageMeters(t *testing.T) {
	now := time.Now()
	q := apiquota.Quota{Status: apiquota.StatusAvailable, Source: apiquota.SourceUpstream, Windows: []apiquota.Window{
		{Title: "5 hour", UsedPercent: 42, RemainingPercent: 58, WindowMinutes: 300, ResetsAt: now.Add(time.Hour)},
		{Title: "7 day", UsedPercent: 21, RemainingPercent: 79, WindowMinutes: 10080},
	}}
	meters := APIQuotaMeters(en, q, now)
	if len(meters) != 2 || meters[0].RemainingPercent != 58 || meters[0].Balance || meters[0].ValueText != "" || !meters[0].HasReset() {
		t.Fatalf("upstream percentages became a balance: %+v", meters)
	}
	if APIBalanceText(q) != "" {
		t.Fatal("upstream quota was formatted as money")
	}
	view := BuildFloating(en, StatusInputs{Prefs: store.Preferences{FloatingWindow: store.FloatingWindow{Providers: []store.Provider{store.ProviderClaude}}}, Claude: ClaudeState{Meters: meters}, Now: now})
	if len(view.Providers) != 1 || len(view.Providers[0].Meters) != 2 || view.Providers[0].Meters[0].RemainingPercent != 58 {
		t.Fatalf("floating view lost upstream windows: %+v", view)
	}
}

func TestAPIBalanceAmountUnits(t *testing.T) {
	cases := []struct {
		unit string
		v    float64
		want string
	}{
		{"USD", 12.345, "$12.35"},
		{"", 1, "$1.00"},
		{"CNY", 3.5, "¥3.50"},
		{"USD", -2, "-$2.00"},
		{"TOKENS", 1_500_000, CompactNumber(1_500_000) + " tokens"},
		{"€", 4, "4.00 €"},
	}
	for _, c := range cases {
		if got := APIBalanceAmount(c.unit, c.v); got != c.want {
			t.Errorf("APIBalanceAmount(%q, %v) = %q, want %q", c.unit, c.v, got, c.want)
		}
	}
}

func TestAPIBalanceMeterWithTotal(t *testing.T) {
	q := apiquota.Quota{Status: apiquota.StatusAvailable, Remaining: f64(30), Used: f64(70), Total: f64(100), Unit: "USD"}
	m, ok := APIBalanceMeter(en, q)
	if !ok || m.UsedPercent != 70 || m.RemainingPercent != 30 || m.ValueText != "$30.00" || !m.Balance {
		t.Fatalf("meter = %+v, ok %v", m, ok)
	}
	if got := APIBalanceDetail(q); got != "$30.00 / $100.00" {
		t.Fatalf("detail = %q", got)
	}
}

func TestAPIBalanceMeterWithoutTotal(t *testing.T) {
	m, ok := APIBalanceMeter(en, apiquota.Quota{Status: apiquota.StatusAvailable, Remaining: f64(5), Unit: "CNY"})
	if !ok || m.RemainingPercent != 100 || m.ValueText != "¥5.00" {
		t.Fatalf("meter = %+v", m)
	}
	m, _ = APIBalanceMeter(en, apiquota.Quota{Status: apiquota.StatusAvailable, Remaining: f64(0), Unit: "CNY"})
	if m.RemainingPercent != 0 || m.Health != HealthRed {
		t.Fatalf("empty balance meter = %+v", m)
	}
	m, _ = APIBalanceMeter(en, apiquota.Quota{Status: apiquota.StatusAvailable, Unlimited: true})
	if m.ValueText != "∞" || m.RemainingPercent != 100 {
		t.Fatalf("unlimited meter = %+v", m)
	}
	if _, ok := APIBalanceMeter(en, apiquota.Quota{Status: apiquota.StatusUnsupported}); ok {
		t.Fatal("no meter without a reading")
	}
}

func TestBalanceMeterCaptionsShowAmount(t *testing.T) {
	m, _ := APIBalanceMeter(en, apiquota.Quota{Status: apiquota.StatusAvailable, Remaining: f64(8), Unit: "USD"})
	now := m.ResetsAt
	if got := CountdownDetail(m, now, en.Language()); got != "$8.00" {
		t.Errorf("countdown detail = %q", got)
	}
	if got := BatteryDetail(m, store.BatteryDetailCountdown, now, en.Language()); got != "$8.00" {
		t.Errorf("battery detail = %q", got)
	}
	if got := MeterDetail(m, store.MetersDetailResetTime, now, en.Language()); got != "$8.00" {
		t.Errorf("meter detail = %q", got)
	}
	if got := percentText(m); got != "$8.00" {
		t.Errorf("percent = %q", got)
	}
	if got := WithShortTitle(en, m).Title; got != "Balance" {
		t.Errorf("short title = %q", got)
	}
}
