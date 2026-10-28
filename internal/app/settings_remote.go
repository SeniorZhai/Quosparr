package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/remotequota"
	"github.com/licoy/quosparr/internal/store"
)

func (u *settingsUI) remoteQuotaGroup(c *ui.Context) {
	l := u.L()
	settingsGroup(c, tx(l, xRemoteSettings), tx(l, xRemoteHint), nil, func(row rowFunc) {
		u.remoteKeyRow(c, row, store.ProviderKimi, &u.kimiKey, true)
		u.remoteKeyRow(c, row, store.ProviderDeepSeek, &u.deepseekKey, false)
		u.remoteKeyRow(c, row, store.ProviderCopilot, &u.copilotToken, false)
		row("", "", func() {
			save := u.opts.SaveRemoteKey
			off := u.s.ReadOnly() || save == nil
			ui.Row(c).Gap(8).Children(func() {
				if PillButton(c, nil, tx(l, xRemoteSave), off || !u.remoteDraftFilled()).Clicked() {
					u.saveRemoteDrafts()
				}
				if PillButton(c, nil, tx(l, xRemoteClear), off).Clicked() {
					u.clearRemoteDrafts()
				}
			})
		})
		if u.remoteNote != "" {
			row("", "", func() {
				col := inkSecondary(c.Theme())
				if u.remoteErr {
					col = sys(c.Theme(), sysRed)
				}
				ui.Text(c, u.remoteNote).FontSize(fsCaption).TextColor(col)
			})
		}
	})
}

func (u *settingsUI) remoteKeyRow(c *ui.Context, row rowFunc, p store.Provider, secret *string, region bool) {
	l := u.L()
	label := tx(l, xRemoteAPIKey)
	if p == store.ProviderCopilot {
		label = tx(l, xRemoteToken)
	}
	detail := ""
	configured := u.opts.RemoteQuota != nil && u.opts.RemoteQuota(p).Configured
	if configured {
		detail = tx(l, xRemoteConfigured)
	}
	row(l.Text(providerKey(p)), detail, func() {
		off := u.s.ReadOnly() || u.opts.SaveRemoteKey == nil
		if configured && !u.remoteEditing[p] {
			if PillButton(c.Key("remote-edit-"+string(p)), nil, tx(l, xRemoteReplace), off).Clicked() {
				if u.remoteEditing == nil {
					u.remoteEditing = make(map[store.Provider]bool)
				}
				u.remoteEditing[p] = true
			}
			return
		}
		ui.Column(c).Gap(6).Children(func() {
			ui.TextInput(c.Key("remote-key-"+string(p)), secret).Password().Width(220).Label(label).Disabled(off)
			if region {
				on := u.kimiIntl
				if ui.Checkbox(c.Key("remote-kimi-intl"), &on, tx(l, xRemoteRegionIntl)).Changed() {
					u.kimiIntl = on
				}
			}
		})
	})
}

func (u *settingsUI) remoteDraftFilled() bool {
	return u.kimiKey != "" || u.deepseekKey != "" || u.copilotToken != ""
}

func (u *settingsUI) saveRemoteDrafts() {
	save := u.opts.SaveRemoteKey
	if save == nil {
		return
	}
	type item struct {
		p      store.Provider
		secret string
		region string
	}
	var items []item
	if u.kimiKey != "" {
		region := remotequota.RegionChina
		if u.kimiIntl {
			region = remotequota.RegionIntl
		}
		items = append(items, item{store.ProviderKimi, u.kimiKey, region})
	}
	if u.deepseekKey != "" {
		items = append(items, item{store.ProviderDeepSeek, u.deepseekKey, ""})
	}
	if u.copilotToken != "" {
		items = append(items, item{store.ProviderCopilot, u.copilotToken, ""})
	}
	l := u.L()
	for _, it := range items {
		if err := save(it.p, it.secret, it.region); err != nil {
			u.remoteErr = true
			u.remoteNote = tx(l, xRemoteFailed)
			return
		}
	}
	u.kimiKey, u.deepseekKey, u.copilotToken = "", "", ""
	u.remoteEditing = nil
	u.remoteErr = false
	u.remoteNote = tx(l, xRemoteSaved)
}

func (u *settingsUI) clearRemoteDrafts() {
	save := u.opts.SaveRemoteKey
	if save == nil {
		return
	}
	region := remotequota.RegionChina
	if u.kimiIntl {
		region = remotequota.RegionIntl
	}
	l := u.L()
	for _, p := range []store.Provider{store.ProviderKimi, store.ProviderDeepSeek, store.ProviderCopilot} {
		reg := ""
		if p == store.ProviderKimi {
			reg = region
		}
		if err := save(p, "", reg); err != nil {
			u.remoteErr = true
			u.remoteNote = tx(l, xRemoteFailed)
			return
		}
	}
	u.kimiKey, u.deepseekKey, u.copilotToken = "", "", ""
	u.remoteEditing = nil
	u.remoteErr = false
	u.remoteNote = tx(l, xRemoteSaved)
}
