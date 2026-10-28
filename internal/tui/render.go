package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
)

// All user and provider text is fitted before adding terminal styling.
func styled(s, color string, enabled bool) string {
	if !enabled {
		return s
	}
	return "\x1b[" + color + "m" + s + "\x1b[0m\x1b[48;5;234m\x1b[38;5;252m"
}

func cells(s string) int {
	n := 0
	for len(s) > 0 {
		_, size := utf8.DecodeRuneInString(s)
		r := s[:size]
		if fit(r, 1) == r {
			n++
		} else if fit(r, 2) == r {
			n += 2
		}
		s = s[size:]
	}
	return n
}

func pad(s string, width int) string {
	s = fit(s, max(0, width))
	return s + strings.Repeat(" ", max(0, width-cells(s)))
}

func paired(left, right string, width int) string {
	right = fit(right, width)
	left = fit(left, max(0, width-cells(right)-1))
	return left + strings.Repeat(" ", max(0, width-cells(left)-cells(right))) + right
}

func renderDashboard(l l10n.Localizer, view View, columns, rows, page int, now time.Time, color bool) string {
	w := max(0, columns-1)
	if w == 0 || rows <= 0 {
		return ""
	}
	dim, bright, accent := "38;5;245", "1;38;5;255", "38;5;114"
	footer := "r " + l.Text(l10n.KeyRefresh) + "   c " + l.TUI("configure") + "   q/Esc " + l.Text(l10n.KeyQuit)
	if w < 34 || rows < 15 {
		lines := []string{styled(fit("Quosparr  ·  "+l.TUI("overview"), w), bright, color)}
		perPage := max(1, rows-3)
		pages := max(1, (len(view.Floating.Providers)+perPage-1)/perPage)
		page = (page%pages + pages) % pages
		for i := page * perPage; i < min(len(view.Floating.Providers), (page+1)*perPage); i++ {
			p := view.Floating.Providers[i]
			value := p.Status
			if len(p.Meters) > 0 {
				value = p.Meters[0].ValueText
				if value == "" {
					value = fmt.Sprintf("%d%%", min(100, max(0, p.Meters[0].RemainingPercent)))
				}
			}
			lines = append(lines, fit(p.Name+"  "+value, w))
		}
		lines = append(lines, fit(fmt.Sprintf("n/p %s %d/%d", l.TUI("page"), page+1, pages), w), fit(footer, w))
		return strings.Join(lines[:min(rows, len(lines))], "\r\n")
	}
	status := l.TUI("standalone") + "  ·  " + l.TUI("auto") + " " + time.Duration(view.IntervalSeconds*int(time.Second)).String()
	if view.Refreshing {
		status = l.TUI("refreshing")
	}
	lines := []string{
		styled(paired("Quosparr", now.Format("15:04:05"), w), bright, color),
		styled(fit(l.TUI("overview")+" · "+l.TUI("remaining")+"  /  "+status, w), dim, color),
		"",
	}
	cols := 1
	if w >= 74 {
		cols = 2
	}
	if w >= 116 {
		cols = 3
	}
	cardWidth := (w - (cols-1)*2) / cols
	cardRows := max(1, (rows-6)/9)
	perPage := cols * cardRows
	pages := max(1, (len(view.Floating.Providers)+perPage-1)/perPage)
	page = (page%pages + pages) % pages
	providers := view.Floating.Providers
	start := min(len(providers), page*perPage)
	end := min(len(providers), start+perPage)
	for offset := start; offset < end; offset += cols {
		cards := make([][]string, 0, cols)
		for i := offset; i < min(end, offset+cols); i++ {
			cards = append(cards, renderCard(l, providers[i], cardWidth, color))
		}
		for row := 0; row < 9; row++ {
			parts := make([]string, len(cards))
			for i, card := range cards {
				parts[i] = card[row]
			}
			lines = append(lines, strings.Join(parts, "  "))
		}
	}
	if len(providers) == 0 {
		lines = append(lines, styled(fit(l.Text(l10n.KeyNotLoaded), w), dim, color))
	}
	for len(lines) < rows-3 {
		lines = append(lines, "")
	}
	last := "—"
	if !view.LastRefresh.IsZero() {
		last = view.LastRefresh.Local().Format("15:04:05")
	}
	next := "—"
	if !view.NextRefresh.IsZero() {
		d := max(0, int(view.NextRefresh.Sub(now).Seconds()+0.999))
		next = fmt.Sprintf("%02d:%02d", d/60, d%60)
	}
	refresh := l.TUI("last") + " " + last + "   ·   " + l.TUI("next") + " " + next
	if view.Refreshing {
		refresh = l.TUI("refreshing") + "   ·   " + l.TUI("last") + " " + last
	}
	lines = append(lines, styled(fit(refresh, w), dim, color))
	pageText := ""
	if pages > 1 {
		pageText = fmt.Sprintf("n/p %s %d/%d", l.TUI("page"), page+1, pages)
	}
	lines = append(lines, styled(paired(footer, pageText, w), accent, color))
	return strings.Join(lines[:min(rows, len(lines))], "\r\n")
}

func renderCard(l l10n.Localizer, p runway.FloatingProvider, width int, color bool) []string {
	inner := width - 4
	border := "38;5;239"
	lines := []string{styled("╭"+strings.Repeat("─", width-2)+"╮", border, color)}
	row := func(s, style string) {
		lines = append(lines, styled("│ ", border, color)+styled(pad(s, inner), style, color)+styled(" │", border, color))
	}
	row(paired(p.Name, p.LiveText, inner), "1;38;5;255")
	for _, m := range p.Meters[:min(2, len(p.Meters))] {
		percent := min(100, max(0, m.RemainingPercent))
		value := m.ValueText
		if value == "" {
			value = fmt.Sprintf("%d%%", percent)
		}
		shade := "38;5;114"
		if percent < 25 {
			shade = "38;5;215"
		}
		if percent < 10 {
			shade = "38;5;203"
		}
		if m.ValueText != "" {
			shade = "38;5;117"
		}
		row(paired(m.Label, value, inner), "1;"+shade)
		if m.ValueText == "" {
			filled := percent * inner / 100
			lines = append(lines, styled("│ ", border, color)+styled(strings.Repeat("━", filled), shade, color)+styled(strings.Repeat("─", inner-filled), border, color)+styled(" │", border, color))
		} else {
			row("", shade)
		}
		reset := ""
		if m.Reset != "" {
			reset = l.TUI("reset") + " " + m.Reset
		}
		row(reset, "38;5;245")
	}
	if len(p.Meters) == 0 {
		text := fit(p.Status, 4096)
		for text != "" && len(lines) < 8 {
			part := fit(text, inner)
			row(part, "38;5;245")
			text = strings.TrimLeft(strings.TrimPrefix(text, part), " ")
		}
	}
	for len(lines) < 8 {
		row("", "0")
	}
	return append(lines, styled("╰"+strings.Repeat("─", width-2)+"╯", border, color))
}
