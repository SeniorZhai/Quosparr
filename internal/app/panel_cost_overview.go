package app

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

const costOverviewTTL = 5 * time.Minute

type panelCostOverviewState struct {
	cancel     context.CancelFunc
	generation uint64
	task       Task
	report     *usage.CostOverview
	loadedAt   time.Time
	day        string
	versions   [3]string
}

func costOverviewVersions(acts PanelActions) (versions [3]string) {
	if acts.PricingVersion != nil {
		for i, provider := range []store.Provider{store.ProviderCodex, store.ProviderClaude, store.ProviderGrok} {
			versions[i] = acts.PricingVersion(provider)
		}
	}
	return
}

func (a *App) cancelCostOverview() {
	s := &a.panelState.costOverview
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = nil
	s.generation++
	s.task.Running = false
}

func (a *App) syncCostOverview(acts PanelActions, now time.Time) {
	s := &a.panelState.costOverview
	if acts.QueryCostOverview == nil {
		return
	}
	if s.day != now.In(time.Local).Format("2006-01-02") || s.versions != costOverviewVersions(acts) {
		s.report = nil
		a.startCostOverview(acts, now)
		return
	}
	if !s.task.Running && (s.loadedAt.IsZero() || now.Sub(s.loadedAt) >= costOverviewTTL) {
		a.startCostOverview(acts, now)
	}
}

func (a *App) startCostOverview(acts PanelActions, now time.Time) {
	if acts.QueryCostOverview == nil {
		return
	}
	a.cancelCostOverview()
	s := &a.panelState.costOverview
	s.day = now.In(time.Local).Format("2006-01-02")
	s.versions = costOverviewVersions(acts)
	s.task = Task{Running: true}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	gen, versions := s.generation, s.versions
	go func() {
		report, err := acts.QueryCostOverview(ctx)
		runOnMain(func() {
			if ctx.Err() != nil || s.generation != gen {
				return
			}
			s.cancel = nil
			s.task = Task{Err: err}
			s.loadedAt = time.Now()
			if versions != costOverviewVersions(acts) {
				s.report, s.loadedAt = nil, time.Time{}
			} else if err == nil {
				s.report = &report
			}
			a.redrawPanel()
		})
	}()
}

func panelCostOverview(c *ui.Context, l l10n.Localizer, s *panelCostOverviewState) {
	t := c.Theme()
	ui.Column(c).Gap(5).Padding(10, 0, 0, 0).Children(func() {
		if s.task.Running {
			ui.Spinner(c).Size(10, 10).Label(tx(l, xPanelLoading))
		} else if s.task.Err != nil {
			ui.Text(c, tx(l, xCostOverviewFailed)).FontSize(10).TextColor(t.Warning)
		}
		ui.Row(c).Gap(6).Children(func() {
			for i, label := range []textKey{xCostOverviewToday, xCostOverviewWeek, xCostOverviewMonth} {
				ui.Column(c.Key(fmt.Sprintf("cost-overview-%d", i))).Grow(1).Basis(0).Gap(2).Padding(6, 8).Radius(radiusControl).Background(mono(t, 0.035, 0.055)).Children(func() {
					ui.Text(c, tx(l, label)).FontSize(10).TextColor(t.TextMuted).SingleLine().Tooltip(tx(l, xCostOverviewInfo))
					value, partial := "—", false
					if s.report != nil {
						period := s.report.Periods[i]
						partial = period.Partial
						value = costMoney(period.EstimatedUSD)
					}
					ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
						ui.Text(c, value).FontSize(15).FontWeight(600).FontFeatures("tnum").SingleLine().Shrink(1).Ellipsis("…").Tooltip(value)
						if partial {
							ui.Text(c, "*").FontSize(11).TextColor(t.Warning).Tooltip(tx(l, xCostOverviewPartial))
						}
					})
				})
			}
		})
	})
}
