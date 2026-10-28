package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/usage"
)

func TestThemeUsesAccessibleBrandAccent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		theme := buildTheme(dark)
		if theme.Accent != ui.Hex("#B3582D") {
			t.Fatalf("dark=%t accent=%v", dark, theme.Accent)
		}
		states := map[string]ui.Color{
			"normal":  theme.Accent,
			"hover":   theme.AccentHover,
			"pressed": theme.AccentPressed,
		}
		for name, color := range states {
			if ratio := contrastRatio(color, theme.AccentText); ratio < 4.5 {
				t.Errorf("dark=%t %s contrast=%.2f", dark, name, ratio)
			}
			if ratio := contrastRatio(brandInk(theme, color), pal(theme).Inset); ratio < 4.5 {
				t.Errorf("dark=%t foreground %s contrast=%.2f", dark, name, ratio)
			}
		}
		selectionAlpha, focusAlpha := float32(0.25), float32(0.5)
		if dark {
			selectionAlpha, focusAlpha = 0.4, 0.6
		}
		if want := theme.Accent.Alpha(selectionAlpha); theme.Selection != want {
			t.Errorf("dark=%t selection=%v, want %v", dark, theme.Selection, want)
		}
		if want := theme.Accent.Alpha(focusAlpha); theme.Focus != want {
			t.Errorf("dark=%t focus=%v, want %v", dark, theme.Focus, want)
		}
	}
}

func TestTokenHeatmapUsesThemeAccent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		theme := buildTheme(dark)
		got := heatLevelColor(theme, usage.HeatmapCell{InRange: true, Level: 4})
		if want := theme.Accent.Alpha(0.92); got != want {
			t.Fatalf("dark=%t heat level color=%v, want %v", dark, got, want)
		}
	}
}

func TestHighlightsFollowSelectedAccent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, color := range []string{"#8659D6", "#168466", "#F5CE60"} {
			theme := buildThemeColor(dark, color)
			ink := brandInk(theme, theme.Accent)
			for tint := sysGreen; tint < sysGray; tint++ {
				if got := sys(theme, tint); got != ink {
					t.Fatalf("dark=%t color=%s tint=%d: %v", dark, color, tint, got)
				}
			}
			for _, health := range []runway.Health{runway.HealthGreen, runway.HealthYellow, runway.HealthRed} {
				if healthFill(theme, health) != theme.Accent || healthInk(theme, health) != ink {
					t.Fatal("quota ignored the selected accent")
				}
			}
			for _, phase := range []expiryPhase{expiryActive, expirySoon, expiryExpired} {
				look := expiryLookOf(theme, phase)
				if look.foreground != ink || look.fill.R != theme.Accent.R || look.fill.G != theme.Accent.G || look.fill.B != theme.Accent.B {
					t.Fatal("expiry ignored the selected accent")
				}
			}
			if look := accentTierLook(theme); look.foreground != ink || look.text[0] != ink {
				t.Fatal("tier ignored the selected accent")
			}
			if contrastRatio(ink, pal(theme).Inset) < 4.5 {
				t.Fatal("highlight text lost contrast")
			}
		}
	}
}
