package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestClaudeQuotaShowsRemainingForEveryWindow(t *testing.T) {
	for _, language := range []l10n.Language{l10n.English, l10n.SimplifiedChinese} {
		l := l10n.New(language)
		now := time.Now()
		snap := runway.Snapshot{Claude: runway.ClaudeState{
			Availability: runway.ClaudeReady,
			Meters: []runway.Meter{
				{Title: runway.ShortWindowTitle(l, runway.FiveHourMinutes), UsedPercent: 38, RemainingPercent: 62, ResetsAt: now.Add(time.Hour)},
				{Title: runway.ShortWindowTitle(l, runway.WeeklyMinutes), UsedPercent: 16, RemainingPercent: 84, ResetsAt: now.Add(5 * 24 * time.Hour), Projection: &runway.BurnProjection{ProjectedUsedPercentAtReset: 30}},
				{Title: "Sonnet", UsedPercent: 25, RemainingPercent: 75},
			},
			ExtraUsage: "$2.00",
		}}
		tt := ui.NewTester(func(c *ui.Context) {
			first := true
			ui.Column(c).Fill().Children(func() {
				panelClaudeQuota(c, l, snap, PanelActions{}, &first, &panelState{}, resetSwitch{}, false)
			})
		}, 400, 300)
		for _, m := range snap.Claude.Meters {
			want := l10n.Format("%d%% %s", m.RemainingPercent, l.Text(l10n.KeyLeft))
			used := l10n.Format("%d%% %s", m.UsedPercent, l.Text(l10n.KeyUsed))
			if !tt.HasText(want) || tt.HasText(used) {
				t.Fatalf("%s quota must show remaining; texts %q", m.Title, tt.Texts())
			}
		}
		if !tt.HasText("$2.00") {
			t.Fatal("extra usage is missing")
		}
		if tt.HasText(l.Text(l10n.KeyBurnRate)) {
			t.Fatal("changing the quota direction added a projection row")
		}
		if dir := os.Getenv("RUNWAY_CLAUDE_QUOTA_QA"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			tt.SetDark(true)
			tt.SetScale(2)
			exportReviewPNG(t, tt, filepath.Join(dir, string(language)+"-remaining.png"))
		}
	}
}

func TestPanelParityQuotaShowsProjectionAndLocalizesLiveReset(t *testing.T) {
	l := l10n.New(l10n.SimplifiedChinese)
	m := runway.Meter{Title: "fixture", RemainingPercent: 70, ResetText: "stale English duration", ResetsAt: time.Now().Add(time.Hour), Projection: &runway.BurnProjection{ProjectedUsedPercentAtReset: 75}}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { meterRows(c, l, []runway.Meter{m}, false, false, resetSwitch{}) })
	}, 368, 200)
	if !tt.HasText(l.Text(l10n.KeyProjectedAtReset)+" 75%") || tt.HasText("stale English duration") || !tt.HasText(l.Text(l10n.KeyNextResetIn)) {
		t.Fatalf("quota projection or live localized reset omitted; texts %q", tt.Texts())
	}
}

func TestPanelParityQuotaDrawsThresholdMarkers(t *testing.T) {
	m := runway.Meter{RemainingPercent: 70}
	render := func() []byte {
		tt := ui.NewTester(func(c *ui.Context) { quotaBar(c, m) }, 368, 20)
		return append([]byte(nil), tt.Image().Pix...)
	}
	plain := render()
	m.MarkerPercents = []int{25, 50, 75}
	if bytes.Equal(plain, render()) {
		t.Fatal("threshold markers did not change the actual quota bar image")
	}
}

func TestQuotaResetLineSwitchesFormatOnClick(t *testing.T) {
	l := l10n.New(l10n.SimplifiedChinese)
	now := time.Now()
	m := runway.Meter{Title: "fixture", RemainingPercent: 57, ResetsAt: now.Add(6*24*time.Hour + 3*time.Hour)}
	format := store.ResetTimeRelative
	st := &panelState{setResetTimeFormat: func(f store.ResetTimeFormat) { format = f }}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { meterRows(c, l, []runway.Meter{m}, false, true, st.resetSwitch(format)) })
	}, 368, 200)
	exact := l.Text(l10n.KeyNextResetIn) + runway.ResetExactTime(m.ResetsAt, now, l10n.SimplifiedChinese, nil)
	if tt.HasText(exact) || !tt.HasText(l.Text(l10n.KeyNextResetIn)) {
		t.Fatalf("relative reset expected; texts %q", tt.Texts())
	}
	label := resetText(l, m.ResetsAt, time.Now(), store.ResetTimeRelative)
	if err := tt.Click(label); err != nil {
		t.Fatalf("click %q: %v; texts %q", label, err, tt.Texts())
	}
	tt.Frame()
	if format != store.ResetTimeExact || !tt.HasText(exact) {
		t.Fatalf("format %q after click; texts %q", format, tt.Texts())
	}
	if err := tt.Click(exact); err != nil {
		t.Fatal(err)
	}
	if format != store.ResetTimeRelative {
		t.Fatalf("second click left format %q", format)
	}
}
