package runway

import (
	"reflect"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/store"
)

func TestFloatingSnapshotExistsWithDesktopWindowDisabled(t *testing.T) {
	prefs := store.DefaultPreferences()
	prefs.FloatingWindow.Enabled = false
	s := New(Options{Preferences: prefs})
	defer s.Close()
	s.publish()
	if got := s.Snapshot().Floating; len(got.Providers) != 1 || got.Providers[0].Provider != store.ProviderCodex {
		t.Fatalf("terminal mirror has no floating view: %+v", got)
	}
}

func TestFloatingCountdownParticipatesInTickUpdates(t *testing.T) {
	s := Snapshot{Floating: FloatingView{Providers: []FloatingProvider{{Provider: store.ProviderClaude,
		Meters: []FloatingMeter{{Reset: "2h"}},
	}}}}
	before := tickKey(s)
	s.Floating.Providers[0].Meters[0].Reset = "1h 59m"
	if tickKey(s) == before {
		t.Fatal("floating-only reset countdown did not trigger publication")
	}
}

func TestFloatingShowsTwoStandardWindowsPerProvider(t *testing.T) {
	weekly := testMeter("每周", 60, WeeklyMinutes, SourceStandard)
	five := testMeter("5小时", 20, FiveHourMinutes, SourceStandard)
	spark := testMeter("Spark", 0, WeeklyMinutes, SourceModelSpecific)
	p := store.DefaultPreferences()
	p.FloatingWindow.Providers = []store.Provider{store.ProviderCodex, store.ProviderClaude}
	v := BuildFloating(zh, StatusInputs{
		Prefs: p, Now: time.Now(),
		CodexText: "80%", CodexMeters: []Meter{weekly, spark, five},
		Claude: ClaudeState{Availability: ClaudeNotLoggedIn},
	})
	if len(v.Providers) != 2 {
		t.Fatalf("providers = %+v", v.Providers)
	}
	codex := v.Providers[0]
	if codex.Provider != store.ProviderCodex || len(codex.Meters) != 2 || codex.Status != "" {
		t.Fatalf("codex = %+v", codex)
	}
	if codex.Meters[0].RemainingPercent != 80 || codex.Meters[1].RemainingPercent != 40 || codex.Meters[0].Reset == "" {
		t.Errorf("codex meters = %+v, want the 5-hour window first", codex.Meters)
	}
	claude := v.Providers[1]
	if len(claude.Meters) != 0 || claude.Status == "" {
		t.Errorf("claude = %+v, want a status without meters", claude)
	}
}

func TestFloatingFollowsThePanelProvider(t *testing.T) {
	p := store.DefaultPreferences()
	p.SelectedProvider = store.ProviderGrok
	monthly := testMeter("每月", 10, MonthlyMinutes, SourceStandard)
	v := BuildFloating(zh, StatusInputs{Prefs: p, Now: time.Now(), Grok: GrokState{Availability: GrokReady, Meters: []Meter{monthly}}})
	if len(v.Providers) != 1 || v.Providers[0].Provider != store.ProviderGrok || len(v.Providers[0].Meters) != 1 {
		t.Errorf("view = %+v", v)
	}
}

func TestFloatingWithOnlyModelSpecificWindowShowsIt(t *testing.T) {
	spark := testMeter("Spark", 30, WeeklyMinutes, SourceModelSpecific)
	if got := floatingMeters([]Meter{spark}, codexPreferredWindows); len(got) != 1 || got[0].Title != "Spark" {
		t.Errorf("meters = %+v", got)
	}
	if got := floatingMeters(nil, codexPreferredWindows); len(got) != 0 {
		t.Errorf("no meters = %+v", got)
	}
}

func TestFloatingPreservesSelectionAndQuotaWhileLive(t *testing.T) {
	for _, style := range store.FloatingStyles() {
		t.Run(string(style), func(t *testing.T) {
			p := store.DefaultPreferences()
			p.FloatingWindow.Style = style
			p.FloatingWindow.Providers = []store.Provider{store.ProviderCodex, store.ProviderClaude}
			in := StatusInputs{
				Prefs: p, Now: time.Now(),
				CodexMeters: []Meter{testMeter("5h", 20, FiveHourMinutes, SourceStandard)},
				Claude: ClaudeState{Availability: ClaudeReady, RemoteUsage: true, Meters: []Meter{
					testMeter("weekly", 60, WeeklyMinutes, SourceStandard),
					testMeter("5h", 30, FiveHourMinutes, SourceStandard),
				}},
				Grok: GrokState{Availability: GrokNotLoggedIn},
			}
			idle := BuildFloating(zh, in)
			in.Live = []LivePart{
				{Provider: store.ProviderClaude, Text: "8 tok/s"},
				{Provider: store.ProviderGrok, Text: "3 tok/s"},
				{Provider: store.ProviderClaude, Text: "8 tok/s"},
				{Provider: store.ProviderGrok, Text: "3 tok/s"},
			}
			v := BuildFloating(zh, in)
			if !v.Live || len(v.Providers) != 3 {
				t.Fatalf("live view = %+v", v)
			}
			if !reflect.DeepEqual(v.Providers[0], idle.Providers[0]) {
				t.Errorf("inactive provider changed: %+v, want %+v", v.Providers[0], idle.Providers[0])
			}
			claude := v.Providers[1]
			if claude.Provider != store.ProviderClaude || claude.Name != "Claude" || claude.LiveText != "8 tok/s" {
				t.Fatalf("claude = %+v", claude)
			}
			claude.LiveText = ""
			if !reflect.DeepEqual(claude, idle.Providers[1]) || len(claude.Meters) != 2 {
				t.Fatalf("live quota metadata changed: %+v, want %+v", claude, idle.Providers[1])
			}
			if claude.Meters[0].RemainingPercent != 70 || claude.Meters[1].RemainingPercent != 40 {
				t.Errorf("quota order = %+v, want 5h then weekly", claude.Meters)
			}
			grok := v.Providers[2]
			if grok.Provider != store.ProviderGrok || grok.LiveText != "3 tok/s" || grok.Status == "" || len(grok.Meters) != 0 {
				t.Errorf("appended provider = %+v, want speed and login status", grok)
			}
			in.Live = nil
			if got := BuildFloating(zh, in); !reflect.DeepEqual(got, idle) {
				t.Errorf("idle view = %+v, want %+v", got, idle)
			}
		})
	}
}

func TestFloatingIgnoresInvalidLiveParts(t *testing.T) {
	for _, style := range store.FloatingStyles() {
		t.Run(string(style), func(t *testing.T) {
			p := store.DefaultPreferences()
			p.FloatingWindow.Style = style
			p.FloatingWindow.Providers = []store.Provider{store.ProviderClaude}
			in := StatusInputs{Prefs: p, Now: time.Now(), Claude: ClaudeState{Availability: ClaudeNotLoggedIn}}
			want := BuildFloating(zh, in)
			for _, part := range []LivePart{
				{Provider: store.ProviderClaude},
				{Provider: "invalid", Text: "8 tok/s"},
				{Provider: store.ProviderKimi, Text: "8 tok/s"},
				{Provider: store.ProviderDeepSeek, Text: "8 tok/s"},
				{Provider: store.ProviderCopilot, Text: "8 tok/s"},
			} {
				t.Run(string(part.Provider), func(t *testing.T) {
					in.Live = []LivePart{part}
					if got := BuildFloating(zh, in); !reflect.DeepEqual(got, want) {
						t.Errorf("invalid live view = %+v, want %+v", got, want)
					}
				})
			}
		})
	}
}
