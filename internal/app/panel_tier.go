package app

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/grok"
)

// tierLook holds the subscription capsule and identity colors.
type tierLook struct {
	foreground ui.Color
	fill       []ui.Color
	stroke     ui.Color
	sheen      ui.Color
	// text is the readable gradient of the identity text.
	text []ui.Color
}

func pick[T any](light bool, a, b T) T {
	if light {
		return a
	}
	return b
}

func plainLook(t *ui.Theme) tierLook {
	ink := inkSecondary(t)
	light := !t.Dark
	return tierLook{
		foreground: ink,
		fill:       []ui.Color{pick(light, ui.RGBA(0, 0, 0, 0.10), ui.RGBA(255, 255, 255, 0.14))},
		stroke:     hairlineColor(t).Alpha(pick[float32](light, 0.9, 0.55) * 1.0),
		text:       []ui.Color{ink},
	}
}

func accentTierLook(t *ui.Theme) tierLook {
	ink := brandInk(t, t.Accent)
	return tierLook{
		foreground: ink,
		fill:       []ui.Color{t.Accent.Alpha(0.12), t.Accent.Alpha(0.28), t.Accent.Alpha(0.12)},
		stroke:     t.Accent.Alpha(0.5),
		sheen:      t.Accent.Alpha(0.35),
		text:       []ui.Color{ink},
	}
}

func codexTierLook(t *ui.Theme, tier codex.Tier) tierLook {
	if tier == codex.TierUnknown {
		return plainLook(t)
	}
	return accentTierLook(t)
}

func grokTierLook(t *ui.Theme, tier grok.TierKind) tierLook {
	if tier == grok.TierUnknown {
		return plainLook(t)
	}
	return accentTierLook(t)
}

func claudeTierLook(t *ui.Theme, tier claude.TierKind) tierLook {
	if tier == claude.TierUnknown {
		return plainLook(t)
	}
	return accentTierLook(t)
}

// fillStops paints a 2- or 3-stop left-to-right gradient into r.
func fillStops(p *ui.Painter, r ui.Rect, stops []ui.Color, radius float32) {
	switch len(stops) {
	case 0:
	case 1:
		p.Fill(r, stops[0], radius)
	case 2:
		p.FillGradient(r, ui.LinearGradient{From: stops[0], To: stops[1], Angle: 100}, radius)
	default:
		// Clip only the gradient halves; each fill owns its rounded shape.
		// The GPU renderer retains only the innermost rounded clip.
		half := r.W / 2
		p.Clip(ui.Rect{X: r.X, Y: r.Y, W: half, H: r.H}, 0, func() {
			p.FillGradient(r, ui.LinearGradient{From: stops[0], To: stops[1], Angle: 100, End: 0.5}, radius)
		})
		p.Clip(ui.Rect{X: r.X + half, Y: r.Y, W: r.W - half, H: r.H}, 0, func() {
			p.FillGradient(r, ui.LinearGradient{From: stops[1], To: stops[2], Angle: 100, Start: 0.5, End: 1}, radius)
		})
	}
}

// tierBadge is SubscriptionTierBadge: a capsule with the label at 10pt
// semibold, 7pt by 3pt padding.
func tierBadge(c *ui.Context, look tierLook, label string) { tierBadgePadded(c, look, label, 3, 7) }

// tierBadgePadded is tierBadge with its own padding; the account cards use
// the tighter 1pt by 5pt of SubscriptionTierTag.
func tierBadgePadded(c *ui.Context, look tierLook, label string, vertical, horizontal float32) {
	reduceMotion := c.Preferences().ReduceMotion
	ui.Row(c).Shrink(0).Padding(vertical, horizontal).Radius(999).AlignItems(ui.Center).Label(label).Draw(func(p *ui.Painter, r ui.Rect) {
		fillStops(p, r, look.fill, r.H/2)
		if look.sheen.A > 0 && !reduceMotion {
			drawTierSheen(p, r, look.sheen)
		}
	}).DrawOver(func(p *ui.Painter, r ui.Rect) {
		p.Stroke(r, look.stroke, r.H/2, 1)
	}).Children(func() {
		ui.Text(c, label).FontSize(fsCaption).FontWeight(600).TextColor(look.foreground).SingleLine()
	})
}

// gradientColor samples the stops at fraction f in [0, 1].
func gradientColor(stops []ui.Color, f float32) ui.Color {
	if len(stops) == 1 {
		return stops[0]
	}
	pos := f * float32(len(stops)-1)
	i := min(len(stops)-2, max(0, int(pos)))
	return stops[i].Mix(stops[i+1], pos-float32(i))
}

// gradientSpans colors a text rune by rune along the gradient.
func gradientSpans(text string, stops []ui.Color, size float32, weight int) []ui.Span {
	if len(stops) == 1 || text == "" {
		return []ui.Span{{Text: text, Size: size, Weight: weight, Color: stops[0]}}
	}
	n := utf8.RuneCountInString(text)
	spans := make([]ui.Span, 0, n)
	i := 0
	for _, r := range text {
		f := float32(i) / float32(max(1, n-1))
		spans = append(spans, ui.Span{Text: string(r), Size: size, Weight: weight, Color: gradientColor(stops, f)})
		i++
	}
	return spans
}

// tierText uses the same accent as its subscription capsule.
func tierText(c *ui.Context, look tierLook, text string) ui.Element {
	return ui.RichText(c, gradientSpans(text, look.text, fsCallout, 600)...).SingleLine().Ellipsis("…").Label(text)
}
