package app

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

// The floating window is drawn by one painter, so the window can take the
// exact size the same layout computes (floatingSize). Sizes are DIPs.

// floatingInk keeps floating text white on a dark surface in both appearances.
type floatingInk struct {
	bg, border, text, muted, track ui.Color
}

func floatingInkFor(_ *ui.Theme) floatingInk {
	return floatingInk{bg: ui.Hex("#1F1F21"), border: ui.RGBA(255, 255, 255, 0.12), text: ui.RGB(255, 255, 255),
		muted: ui.RGBA(255, 255, 255, 0.7), track: ui.RGBA(255, 255, 255, 0.14)}
}

func (k floatingInk) level(h runway.Health, text bool) ui.Color {
	if text {
		return k.text
	}
	return trayLevelColor(h, true)
}

func floatingFont(size float32, weight int, family string) ui.Font {
	return ui.Font{Family: family, Size: size, Weight: weight, Features: "tnum"}
}

func floatingTextWidth(s string, f ui.Font) float32 {
	var w float32
	for _, g := range ui.Shape(s, f) {
		w = max(w, g.X+g.Advance)
	}
	return float32(math.Ceil(float64(w)))
}

// floatingText draws s with its vertical center at cy, from x, clipped to
// maxW when it is positive.
func floatingText(p *ui.Painter, s string, f ui.Font, x, cy, maxW float32, c ui.Color) {
	if s == "" {
		return
	}
	if maxW > 0 {
		for w := floatingTextWidth(s, f); w > maxW+0.01 && len([]rune(s)) > 1; w = floatingTextWidth(s, f) {
			r := []rune(strings.TrimSuffix(s, "…"))
			s = strings.TrimSpace(string(r[:len(r)-1])) + "…"
		}
	}
	m := f.Metrics()
	p.Glyphs(ui.Shape(s, f), x, cy-(m.Ascent+m.Descent)/2+m.Ascent, c)
}

func percentText(m runway.FloatingMeter) string {
	if m.ValueText != "" {
		return m.ValueText
	}
	return strconv.Itoa(m.RemainingPercent) + "%"
}

func floatingProviderIcon(p *ui.Painter, prov store.Provider, r ui.Rect, k floatingInk) {
	if icon := trayProviderIcon(prov); icon != nil {
		p.Icon(icon, r, k.text)
	}
}

// floatingCornerRadius is the window card's corner radius; zero where the
// system rounds the window itself (Windows 11) or cannot clip it (Windows 10).
func floatingCornerRadius(style store.FloatingStyle, h float32) float32 {
	if !floatingTransparent() {
		return 0
	}
	return floatingStyleRadius(style, h)
}

// floatingStyleRadius is the corner radius the style is designed with.
func floatingStyleRadius(style store.FloatingStyle, h float32) float32 {
	switch style {
	case store.FloatingStyleCapsule:
		return h / 2
	case store.FloatingStyleCard, store.FloatingStyleRings:
		return 14
	}
	return 12
}

// floatingSize is the window content size for the style and view.
func floatingSize(style store.FloatingStyle, v runway.FloatingView, family string) (float32, float32) {
	switch style {
	case store.FloatingStyleCard:
		return cardSize(v, family)
	case store.FloatingStyleRings:
		return ringsSize(v, family)
	case store.FloatingStyleBattery:
		return batterySize(v, family)
	}
	return capsuleSize(v, family)
}

// drawFloating paints the whole window: the card with corners of radius,
// then the style's content.
func drawFloating(p *ui.Painter, r ui.Rect, style store.FloatingStyle, v runway.FloatingView, k floatingInk, family string, radius float32) {
	p.Fill(r, k.bg, radius)
	if radius > 0 {
		p.Stroke(r, k.border, radius, 1)
	}
	switch style {
	case store.FloatingStyleCard:
		drawCard(p, r, v, k, family)
	case store.FloatingStyleRings:
		drawRings(p, r, v, k, family)
	case store.FloatingStyleBattery:
		drawBatteries(p, r, v, k, family)
	default:
		drawCapsule(p, r, v, k, family)
	}
}

// ---- Capsule: the logo and quota values, with the speed below them.

const (
	capsuleH       = 42
	capsulePad     = 13
	capsuleGap     = 21 // between providers, with a hairline in the middle
	capsuleStatusW = 90
)

func capsuleParts(item runway.FloatingProvider, family string) (main, second string, mainW, secondW float32) {
	if len(item.Meters) == 0 {
		w := min(capsuleStatusW, floatingTextWidth(item.Status, floatingFont(12, 500, family)))
		return item.Status, "", w, 0
	}
	main = percentText(item.Meters[0])
	mainW = floatingTextWidth(main, floatingFont(14, 650, family))
	if len(item.Meters) > 1 {
		second = percentText(item.Meters[1])
		secondW = floatingTextWidth(second, floatingFont(11, 500, family))
	}
	return main, second, mainW, secondW
}

func capsuleItemWidth(item runway.FloatingProvider, family string) float32 {
	_, second, mw, sw := capsuleParts(item, family)
	w := mw
	if second != "" {
		w += 5 + sw
	}
	return 22 + max(w, floatingTextWidth(item.LiveText, floatingFont(11, 600, family)))
}

func capsuleSize(v runway.FloatingView, family string) (float32, float32) {
	w := float32(2 * capsulePad)
	for i, item := range v.Providers {
		if i > 0 {
			w += capsuleGap
		}
		w += capsuleItemWidth(item, family)
	}
	return max(w, 64), capsuleH
}

func drawCapsule(p *ui.Painter, r ui.Rect, v runway.FloatingView, k floatingInk, family string) {
	x, cy := r.X+capsulePad, r.Y+r.H/2
	for i, item := range v.Providers {
		if i > 0 {
			p.Fill(ui.Rect{X: x + capsuleGap/2 - 0.5, Y: cy - 8, W: 1, H: 16}, k.track, 0)
			x += capsuleGap
		}
		floatingProviderIcon(p, item.Provider, ui.Rect{X: x, Y: cy - 8, W: 16, H: 16}, k)
		x += 22
		main, second, mw, sw := capsuleParts(item, family)
		w := capsuleItemWidth(item, family) - 22
		qw := mw
		if second != "" {
			qw += 5 + sw
		}
		tx, ty := x+(w-qw)/2, cy
		if item.LiveText != "" {
			ty -= 8
			f := floatingFont(11, 600, family)
			floatingText(p, item.LiveText, f, x+(w-floatingTextWidth(item.LiveText, f))/2, cy+9, 0, k.text)
		}
		if len(item.Meters) == 0 {
			floatingText(p, main, floatingFont(12, 500, family), tx, ty, capsuleStatusW, k.muted)
		} else {
			floatingText(p, main, floatingFont(14, 650, family), tx, ty, 0, k.text)
			if second != "" {
				floatingText(p, second, floatingFont(11, 500, family), tx+mw+5, ty+1, 0, k.muted)
			}
		}
		x += w
	}
}

// ---- Card: a block per provider, a labeled bar per window.

const (
	cardW       = 232
	cardPad     = 12
	cardHeaderH = 22
	cardRowH    = 30
	cardGap     = 13 // between providers, with a hairline in the middle
)

func cardBlockHeight(item runway.FloatingProvider) float32 {
	return cardHeaderH + float32(max(1, len(item.Meters)))*cardRowH
}

func cardSize(v runway.FloatingView, family string) (float32, float32) {
	w, h := float32(cardW), float32(2*cardPad)-4
	for i, item := range v.Providers {
		if i > 0 {
			h += cardGap
		}
		h += cardBlockHeight(item)
		if item.LiveText != "" {
			w = max(w, 2*cardPad+20+floatingTextWidth(item.Name, floatingFont(12.5, 600, family))+12+
				floatingTextWidth(item.LiveText, floatingFont(11.5, 600, family)))
		}
	}
	return w, h
}

func drawCard(p *ui.Painter, r ui.Rect, v runway.FloatingView, k floatingInk, family string) {
	x, w := r.X+cardPad, r.W-2*cardPad
	y := r.Y + cardPad - 2
	for i, item := range v.Providers {
		if i > 0 {
			p.Fill(ui.Rect{X: x, Y: y + cardGap/2 - 0.5, W: w, H: 1}, k.track, 0)
			y += cardGap
		}
		cy := y + cardHeaderH/2
		floatingProviderIcon(p, item.Provider, ui.Rect{X: x, Y: cy - 7, W: 14, H: 14}, k)
		nameW := w - 20
		if item.LiveText != "" {
			f := floatingFont(11.5, 600, family)
			sw := floatingTextWidth(item.LiveText, f)
			floatingText(p, item.LiveText, f, x+w-sw, cy, 0, k.text)
			nameW -= sw + 12
		}
		floatingText(p, item.Name, floatingFont(12.5, 600, family), x+20, cy, nameW, k.text)
		y += cardHeaderH
		if len(item.Meters) == 0 {
			floatingText(p, item.Status, floatingFont(12, 500, family), x, y+cardRowH/2-2, w, k.muted)
			y += cardRowH
			continue
		}
		for _, m := range item.Meters {
			pct := percentText(m)
			pf := floatingFont(13, 650, family)
			pw := floatingTextWidth(pct, pf)
			label := m.Label
			if m.Reset != "" {
				label += " · " + m.Reset
			}
			ly := y + 8
			floatingText(p, label, floatingFont(11, 500, family), x, ly, w-pw-8, k.muted)
			floatingText(p, pct, pf, x+w-pw, ly, 0, k.level(m.Health, true))
			bar := ui.Rect{X: x, Y: y + 18, W: w, H: 5}
			p.Fill(bar, k.track, 2.5)
			if lv := float32(min(100, max(0, m.RemainingPercent))) / 100; lv > 0 {
				bar.W = max(5, bar.W*lv)
				p.Fill(bar, k.level(m.Health, false), 2.5)
			}
			y += cardRowH
		}
	}
}

// ---- Rings: concentric windows, with the values and speed below them.

const (
	ringsPad    = 10
	ringsCellW  = 52
	ringsGap    = 18
	ringRadius  = 22
	ringsH      = 110
	ringsSpeedH = 20
)

func ringsCaption(item runway.FloatingProvider) string {
	if len(item.Meters) == 0 {
		return item.Status
	}
	parts := make([]string, len(item.Meters))
	for i, m := range item.Meters {
		parts[i] = percentText(m)
	}
	return strings.Join(parts, " · ")
}

func ringsItemWidth(item runway.FloatingProvider, family string) float32 {
	w := floatingTextWidth(ringsCaption(item), floatingFont(12, 600, family))
	if len(item.Meters) == 0 {
		w = min(capsuleStatusW, w)
	}
	return max(ringsCellW, w, floatingTextWidth(item.Name, floatingFont(12, 600, family)),
		floatingTextWidth(item.LiveText, floatingFont(12, 650, family)))
}

func ringsSize(v runway.FloatingView, family string) (float32, float32) {
	w, h := float32(2*ringsPad), float32(ringsH)
	for i, item := range v.Providers {
		if i > 0 {
			w += ringsGap
		}
		w += ringsItemWidth(item, family)
		if item.LiveText != "" {
			h = ringsH + ringsSpeedH
		}
	}
	return max(w, 2*ringsPad+ringsCellW), h
}

func drawRings(p *ui.Painter, r ui.Rect, v runway.FloatingView, k floatingInk, family string) {
	x := r.X + ringsPad
	for _, item := range v.Providers {
		w := ringsItemWidth(item, family)
		cx, cy := x+w/2, r.Y+58
		f := floatingFont(12, 600, family)
		floatingText(p, item.Name, f, cx-floatingTextWidth(item.Name, f)/2, r.Y+ringsPad+8, 0, k.text)
		for i := range max(1, len(item.Meters)) {
			radius, stroke := float32(ringRadius), float32(4)
			if i > 0 {
				radius, stroke = 15, 3.5
			}
			p.StrokePath((&ui.Path{}).Circle(cx, cy, radius), stroke, k.track)
			if i < len(item.Meters) {
				m := item.Meters[i]
				ringArc(p, cx, cy, radius, stroke, float64(min(100, max(0, m.RemainingPercent)))/100, k.level(m.Health, false))
			}
		}
		floatingProviderIcon(p, item.Provider, ui.Rect{X: cx - 8, Y: cy - 8, W: 16, H: 16}, k)
		caption := ringsCaption(item)
		cw, color := floatingTextWidth(caption, f), k.text
		if len(item.Meters) == 0 {
			cw, color = min(capsuleStatusW, cw), k.muted
		}
		floatingText(p, caption, f, cx-cw/2, r.Y+91, cw, color)
		f = floatingFont(12, 650, family)
		floatingText(p, item.LiveText, f, cx-floatingTextWidth(item.LiveText, f)/2, r.Y+ringsH, 0, k.text)
		x += w + ringsGap
	}
}

// ringArc strokes the remaining fraction clockwise from the top.
func ringArc(p *ui.Painter, cx, cy, radius, width float32, level float64, c ui.Color) {
	if level <= 0 {
		return
	}
	path := &ui.Path{}
	steps := max(2, int(math.Ceil(level*72)))
	for i := 0; i <= steps; i++ {
		a := -math.Pi/2 + level*2*math.Pi*float64(i)/float64(steps)
		x, y := cx+radius*float32(math.Cos(a)), cy+radius*float32(math.Sin(a))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	p.StrokePath(path, width, c)
}

// ---- Battery: a battery per window, the percentage inside.

const (
	batteryPad  = 12
	batteryRowH = 30
	batteryW    = 52
	batteryH    = 20
	batteryGap  = 8
)

func batterySize(v runway.FloatingView, family string) (float32, float32) {
	w := float32(0)
	h := float32(max(1, len(v.Providers))*batteryRowH + 2*batteryPad - 6)
	for _, item := range v.Providers {
		row := float32(16 + 10)
		if len(item.Meters) == 0 {
			row += min(capsuleStatusW, floatingTextWidth(item.Status, floatingFont(12, 500, family)))
		} else {
			for _, m := range item.Meters {
				row += batteryMeterWidth(m, family) + 4
			}
			row += float32(len(item.Meters)-1) * batteryGap
		}
		w = max(w, row)
		if item.LiveText != "" {
			w = max(w, 26+floatingTextWidth(item.LiveText, floatingFont(11, 600, family)))
			h += 16
		}
	}
	return max(w+2*batteryPad, 80), h
}

func batteryMeterWidth(m runway.FloatingMeter, family string) float32 {
	return max(batteryW, floatingTextWidth(percentText(m), floatingFont(11, 650, family))+10)
}

func drawBatteries(p *ui.Painter, r ui.Rect, v runway.FloatingView, k floatingInk, family string) {
	y := r.Y + batteryPad - 3
	for _, item := range v.Providers {
		cy := y + batteryRowH/2
		x := r.X + batteryPad
		floatingProviderIcon(p, item.Provider, ui.Rect{X: x, Y: cy - 8, W: 16, H: 16}, k)
		x += 26
		if len(item.Meters) == 0 {
			floatingText(p, item.Status, floatingFont(12, 500, family), x, cy, capsuleStatusW, k.muted)
		}
		for i, m := range item.Meters {
			if i > 0 {
				x += batteryGap
			}
			w := batteryMeterWidth(m, family)
			drawFloatingBattery(p, ui.Rect{X: x, Y: cy - batteryH/2, W: w, H: batteryH}, m, k, family)
			x += w + 4
		}
		y += batteryRowH
		if item.LiveText != "" {
			f := floatingFont(11, 600, family)
			w := r.W - 2*batteryPad - 26
			floatingText(p, item.LiveText, f, r.X+batteryPad+26+(w-floatingTextWidth(item.LiveText, f))/2, y+5, 0, k.text)
			y += 16
		}
	}
}

func drawFloatingBattery(p *ui.Painter, body ui.Rect, m runway.FloatingMeter, k floatingInk, family string) {
	p.Stroke(body, k.muted, 5, 1.4)
	p.Fill(ui.Rect{X: body.X + body.W + 1.2, Y: body.Y + body.H*0.32, W: 2.4, H: body.H * 0.36}, k.muted, 1)
	track := ui.Rect{X: body.X + 2.5, Y: body.Y + 2.5, W: body.W - 5, H: body.H - 5}
	level := float32(min(100, max(0, m.RemainingPercent))) / 100
	filled := track
	filled.W = min(track.W, max(3, track.W*level))
	if level > 0 {
		p.Fill(filled, k.level(m.Health, false), 3)
	}
	text := percentText(m)
	f := floatingFont(11, 650, family)
	tw := floatingTextWidth(text, f)
	tx, cy := body.X+(body.W-tw)/2, body.Y+body.H/2
	// One color across the fill edge: the level colors are bright enough for
	// the dark text of the light card and the white text of the dark one.
	floatingText(p, text, f, tx, cy, 0, k.text)
}

// floatingTooltip spells out every window, as the drawing has no labels in
// the compact styles.
func floatingTooltip(v runway.FloatingView) string {
	lines := make([]string, 0, len(v.Providers))
	for _, item := range v.Providers {
		parts := make([]string, 0, len(item.Meters)+1)
		if item.LiveText != "" {
			parts = append(parts, item.LiveText)
		}
		if len(item.Meters) == 0 && item.Status != "" {
			parts = append(parts, item.Status)
		}
		for _, m := range item.Meters {
			part := strings.TrimSpace(m.Label + " " + percentText(m))
			if m.Reset != "" {
				part += " (" + m.Reset + ")"
			}
			parts = append(parts, part)
		}
		lines = append(lines, item.Name+": "+strings.Join(parts, " · "))
	}
	return strings.Join(lines, "\n")
}
