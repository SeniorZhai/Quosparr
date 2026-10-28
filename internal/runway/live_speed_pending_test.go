package runway

import (
	"strings"
	"testing"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

func TestGrokPendingSpeedInEveryLanguage(t *testing.T) {
	p := store.DefaultPreferences()
	p.SelectedProvider = store.ProviderGrok
	p.SetStatusBarGroups([][]store.Provider{{store.ProviderGrok}})
	p.FloatingWindow.Providers = []store.Provider{store.ProviderGrok}
	in := StatusInputs{Prefs: p, Live: []LivePart{{Provider: store.ProviderGrok, Pending: true}}}
	for _, lang := range l10n.Languages() {
		t.Run(string(lang), func(t *testing.T) {
			lc := l10n.New(lang)
			want := lc.Text(l10n.KeyLiveSpeedPending)
			for _, style := range store.StatusBarStyles() {
				in.Prefs.StatusBarDisplayStyle = style
				v := BuildStatusBar(lc, in)
				if len(v.Components) != 1 || v.Components[0].LiveText != want || v.Components[0].Cells[0].LiveText != want || !strings.Contains(v.Tooltip, want) {
					t.Fatalf("%s pending status bar = %+v", style, v)
				}
			}
			v := BuildFloating(lc, in)
			if !v.Live || len(v.Providers) != 1 || v.Providers[0].LiveText != want {
				t.Fatalf("pending floating window = %+v", v)
			}
		})
	}
	in.Live[0].Text = "50 tok/s"
	if got := BuildFloating(zh, in).Providers[0].LiveText; got != "50 tok/s" {
		t.Fatalf("pending replaced a rate: %q", got)
	}
	in.Live = nil
	if got := BuildFloating(zh, in); got.Live || got.Providers[0].LiveText != "" {
		t.Fatalf("pending survived idle: %+v", got)
	}
}
