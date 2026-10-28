package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
	"github.com/licoy/quosparr/internal/usage/decimal"
)

func costOverviewFixture() *usage.CostOverview {
	report := &usage.CostOverview{}
	for i, value := range []string{"12.34", "567.89", "12345.67"} {
		amount := decimal.MustParse(value)
		report.Periods[i] = usage.CostOverviewPeriod{EstimatedUSD: &amount, Tokens: int64(i+1) * 1000}
	}
	return report
}

func TestPanelCostOverviewFitsThreeColumnsInEveryLanguage(t *testing.T) {
	for _, language := range l10n.Languages() {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dark=%v", language, dark), func(t *testing.T) {
				a, tt := newPanel(t, func(p *store.Preferences) {
					p.Language = store.Language(language)
				}, panelFixture(), PanelActions{})
				a.panelState.costOverview.report = costOverviewFixture()
				tt.SetSize(400, 560)
				tt.SetDark(dark)
				tt.Frame()
				l := a.L()
				header, ok := tt.Find("Quosparr")
				if !ok {
					t.Fatal("application header missing")
				}
				var previousLabel, previousValue ui.Rect
				for i, key := range []textKey{xCostOverviewToday, xCostOverviewWeek, xCostOverviewMonth} {
					label, found := tt.Find(tx(l, key))
					valueText := []string{"$12.34", "$567.89", "$12345.67"}[i]
					value, foundValue := tt.Find(valueText)
					if !found || !foundValue || !hasExact(tt, valueText) {
						t.Fatalf("column %d missing: label=%v value=%v", i, found, foundValue)
					}
					if label.X < 16 || value.X < 16 || label.X+label.W > 384 || value.X+value.W > 384 || value.W+1 < floatingTextWidth(valueText, floatingFont(15, 600, "")) {
						t.Fatalf("column %d clipped: label=%+v value=%+v", i, label, value)
					}
					if header.Y+header.H > label.Y || label.Y+label.H > value.Y {
						t.Fatalf("column %d overlaps vertically: header=%+v label=%+v value=%+v", i, header, label, value)
					}
					if i > 0 && (label.X < previousLabel.X+previousLabel.W || value.X < previousValue.X+previousValue.W || label.Y != previousLabel.Y || value.Y != previousValue.Y) {
						t.Fatalf("column %d overlaps or wraps: label=%+v value=%+v", i, label, value)
					}
					previousLabel, previousValue = label, value
				}
				if os.Getenv("RUNWAY_COST_OVERVIEW_QA") != "" {
					dir := filepath.Join("..", "..", ".build", "v1-cost-overview-qa")
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					tt.SetScale(2)
					exportReviewPNG(t, tt, filepath.Join(dir, fmt.Sprintf("%s-dark-%v.png", language, dark)))
				}
			})
		}
	}
}

func TestPanelCostOverviewMissingPartialAndErrorStates(t *testing.T) {
	for _, language := range l10n.Languages() {
		t.Run(string(language), func(t *testing.T) {
			l := l10n.New(language)
			state := panelCostOverviewState{}
			tt := ui.NewTester(func(c *ui.Context) {
				applyTheme(c)
				ui.Column(c).Fill().Padding(16).Children(func() { panelCostOverview(c, l, &state) })
			}, 400, 220)
			dashes := 0
			for _, text := range tt.Texts() {
				if text == "—" {
					dashes++
				}
			}
			if dashes != 3 || hasExact(tt, "$0.00") {
				t.Fatalf("missing data shown as a cost: %q", tt.Texts())
			}
			state.report = costOverviewFixture()
			state.report.Periods[2].Partial = true
			state.task.Err = errors.New("internal diagnostic must not appear")
			tt.Frame()
			if !hasExact(tt, tx(l, xCostOverviewFailed)) || tt.HasText(state.task.Err.Error()) || !hasExact(tt, "$12345.67") {
				t.Fatalf("failed refresh lost its previous total or exposed the error: %q", tt.Texts())
			}
			label, _ := tt.Find(tx(l, xCostOverviewToday))
			failure, _ := tt.Find(tx(l, xCostOverviewFailed))
			if failure.X < 16 || failure.X+failure.W > 384 || failure.Y+failure.H > label.Y {
				t.Fatalf("failure status overlaps or leaves header: label=%+v failure=%+v", label, failure)
			}
			star, ok := tt.Find("*")
			value, _ := tt.Find("$12345.67")
			if !ok || star.X < value.X+value.W || star.X+star.W > 384 {
				t.Fatalf("partial marker missing or overlaps: value=%+v star=%+v", value, star)
			}
			tt.Move(star.X+star.W/2, star.Y+star.H/2)
			tt.Advance(time.Second)
			if !tt.HasText(tx(l, xCostOverviewPartial)) {
				t.Fatalf("partial total has no explanation: %q", tt.Texts())
			}
		})
	}
}

func TestPanelCostOverviewSurvivesProviderChanges(t *testing.T) {
	a, tt := newPanel(t, func(p *store.Preferences) {
		p.PanelProviders = []store.Provider{store.ProviderCodex, store.ProviderClaude, store.ProviderGrok}
	}, panelFixture(), PanelActions{})
	a.SetPanelActions(PanelActions{SelectProvider: func(provider store.Provider) { a.settings.SetSelectedProvider(provider) }})
	report := costOverviewFixture()
	a.panelState.costOverview.report = report
	tt.Frame()
	for _, provider := range []store.Provider{store.ProviderClaude, store.ProviderGrok, store.ProviderCodex} {
		name := a.L().Text(providerKey(provider))
		if err := tt.Click(name); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.settings.Prefs().SelectedProvider != provider || a.panelState.costOverview.report != report {
			t.Fatalf("switch to %s cleared or replaced the shared overview", provider)
		}
		for _, value := range []string{"$12.34", "$567.89", "$12345.67"} {
			amount, ok := tt.Find(value)
			tab, foundTab := tt.Find(name)
			if !ok || !foundTab || amount.Y+amount.H > tab.Y {
				t.Fatalf("%s overview missing above tabs: value=%+v tab=%+v", provider, amount, tab)
			}
		}
	}
}
