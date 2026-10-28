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
	in := StatusInputs{Prefs: p}
	for _, lang := range l10n.Languages() {
		t.Run(string(lang), func(t *testing.T) {
			lc := l10n.New(lang)
			for _, tc := range []struct {
				name string
				part LivePart
				want string
			}{
				{"pending", LivePart{Pending: true}, lc.Text(l10n.KeyLiveSpeedPending)},
				{"last", LivePart{Text: "50 tok/s", Last: true}, lc.Textf(l10n.KeyLiveSpeedLast, "50 tok/s")},
				{"pending with rate", LivePart{Text: "50 tok/s", Pending: true}, "50 tok/s"},
				{"pending with last rate", LivePart{Text: "50 tok/s", Pending: true, Last: true}, lc.Textf(l10n.KeyLiveSpeedLast, "50 tok/s")},
			} {
				t.Run(tc.name, func(t *testing.T) {
					tc.part.Provider = store.ProviderGrok
					in.Live = []LivePart{tc.part}
					for _, style := range store.StatusBarStyles() {
						in.Prefs.StatusBarDisplayStyle = style
						v := BuildStatusBar(lc, in)
						if len(v.Components) != 1 || v.Components[0].LiveText != tc.want || v.Components[0].Cells[0].LiveText != tc.want || !strings.Contains(v.Tooltip, tc.want) {
							t.Fatalf("%s status bar = %+v", style, v)
						}
					}
					v := BuildFloating(lc, in)
					if !v.Live || len(v.Providers) != 1 || v.Providers[0].LiveText != tc.want {
						t.Fatalf("floating window = %+v", v)
					}
				})
			}
		})
	}
	in.Live = nil
	if got := BuildFloating(zh, in); got.Live || got.Providers[0].LiveText != "" {
		t.Fatalf("pending survived idle: %+v", got)
	}
}
