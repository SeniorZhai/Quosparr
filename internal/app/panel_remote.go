package app

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/remotequota"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

type remoteKeyDraft struct {
	secret string
	intl   bool
	note   string
	err    bool
}

func (st *panelState) remoteDraft(p store.Provider) *remoteKeyDraft {
	if st.remoteKeys == nil {
		st.remoteKeys = map[store.Provider]*remoteKeyDraft{}
	}
	d := st.remoteKeys[p]
	if d == nil {
		d = &remoteKeyDraft{}
		st.remoteKeys[p] = d
	}
	return d
}

func panelRemoteQuota(c *ui.Context, l l10n.Localizer, prefs store.Preferences, snap runway.Snapshot, acts PanelActions, first *bool, st *panelState, rs resetSwitch) {
	p := prefs.SelectedProvider
	q := snap.Remote[p]
	refreshing := sectionRefreshing(snap, runway.SectionQuota)
	sectionBlock(c, first, func() {
		drawSectionHead(c, l, st, sectionSpec{
			title: l.Text(providerKey(p)), refreshing: refreshing, refresh: sectionRefresh(acts, runway.SectionQuota),
		})
		if len(q.Meters) == 0 {
			if msg := remoteStatusLine(l, q); msg != "" && !refreshing {
				dimText(c, msg).MaxLines(3)
			} else {
				sectionEmpty(c, l, refreshing || q.Code == "")
			}
		} else {
			remoteMeterRows(c, l, q, rs)
		}
		if !q.Configured {
			remoteKeyForm(c, l, p, q, acts, st.remoteDraft(p))
		}
	})
}

func remoteMeterRows(c *ui.Context, l l10n.Localizer, q runway.RemoteQuota, rs resetSwitch) {
	now := time.Now()
	ui.Column(c).Gap(12).Children(func() {
		for i, m := range q.Meters {
			if m.Balance {
				remoteBalanceRow(c.Key("remote-balance-"+strconv.Itoa(i)), l, m)
				continue
			}
			quotaRow(c.Key("remote-quota-"+strconv.Itoa(i)), l, m, now, false, rs)
		}
		for _, line := range q.Lines {
			ui.Row(c).Gap(8).Children(func() {
				dimText(c, remoteLineTitle(l, line.Title)).Grow(1)
				dimText(c, line.Value)
			})
		}
	})
}

func remoteBalanceRow(c *ui.Context, l l10n.Localizer, m runway.Meter) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, m.Title).FontSize(fsCallout).FontWeight(600).Grow(1).Basis(0)
		text := m.ValueText
		if text == "" {
			text = l10n.Format("%d%%", m.RemainingPercent)
		}
		ui.Text(c, text).FontSize(fsCallout).FontWeight(600).FontFeatures("tnum").TextColor(healthInk(t, m.Health)).SingleLine()
	})
}

func remoteKeyForm(c *ui.Context, l l10n.Localizer, p store.Provider, q runway.RemoteQuota, acts PanelActions, d *remoteKeyDraft) {
	t := c.Theme()
	label := tx(l, xRemoteAPIKey)
	if p == store.ProviderCopilot {
		label = tx(l, xRemoteToken)
	}
	ui.Column(c).Gap(8).Padding(8, 0, 0, 0).Children(func() {
		dimText(c, tx(l, xRemoteHint)).MaxLines(3)
		if q.Configured {
			dimText(c, tx(l, xRemoteConfigured))
		}
		ui.TextInput(c.Key("remote-secret-"+string(p)), &d.secret).Password().Placeholder(label).Label(label)
		if p == store.ProviderKimi {
			on := d.intl
			if ui.Checkbox(c.Key("remote-intl"), &on, tx(l, xRemoteRegionIntl)).Changed() {
				d.intl = on
			}
		}
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, tx(l, xRemoteSave)).Disabled(acts.SaveRemoteKey == nil || d.secret == "").Clicked() {
				region := remotequota.RegionChina
				if d.intl {
					region = remotequota.RegionIntl
				}
				err := acts.SaveRemoteKey(p, d.secret, region)
				d.err = err != nil
				if err != nil {
					d.note = tx(l, xRemoteFailed)
				} else {
					d.secret = ""
					d.note = tx(l, xRemoteSaved)
				}
			}
			if ui.Button(c, tx(l, xRemoteClear)).Disabled(acts.SaveRemoteKey == nil || !q.Configured).Clicked() {
				region := ""
				if p == store.ProviderKimi {
					region = remotequota.RegionChina
					if d.intl {
						region = remotequota.RegionIntl
					}
				}
				err := acts.SaveRemoteKey(p, "", region)
				d.err = err != nil
				if err != nil {
					d.note = tx(l, xRemoteFailed)
				} else {
					d.secret = ""
					d.note = tx(l, xRemoteSaved)
				}
			}
		})
		if d.note != "" {
			col := inkSecondary(t)
			if d.err {
				col = sys(t, sysRed)
			}
			ui.Text(c, d.note).FontSize(fsCaption).TextColor(col)
		}
	})
}

func remoteIdentityRow(c *ui.Context, l l10n.Localizer, prefs store.Preferences, snap runway.Snapshot) {
	q := snap.Remote[prefs.SelectedProvider]
	name := q.Plan
	if name == "" {
		name = l.Text(providerKey(prefs.SelectedProvider))
	}
	row := identityPill(c, name)
	row.Children(func() {
		tierBadge(c, plainLook(c.Theme()), l.Text(providerKey(prefs.SelectedProvider)))
		ui.Text(c, name).FontSize(fsCallout).FontWeight(600).SingleLine().Ellipsis("…").MaxWidth(220)
	})
}

func remoteStatusLine(l l10n.Localizer, q runway.RemoteQuota) string {
	switch q.Code {
	case remotequota.CodeUnconfigured:
		return tx(l, xRemoteUnconfigured)
	case remotequota.CodeUnauthorized:
		return tx(l, xRemoteUnauthorized)
	case remotequota.CodeUnavailable:
		return tx(l, xRemoteUnavailable)
	case remotequota.CodeFailed:
		return tx(l, xRemoteFailed)
	default:
		return ""
	}
}

func remoteLineTitle(l l10n.Localizer, id string) string {
	switch id {
	case remotequota.LineGranted:
		return tx(l, xRemoteGranted)
	case remotequota.LineTopped:
		return tx(l, xRemoteTopped)
	case remotequota.LineMonthCode:
		return tx(l, xRemoteCodeShare)
	case remotequota.LineDeficit:
		return tx(l, xRemoteDeficit)
	default:
		return id
	}
}
