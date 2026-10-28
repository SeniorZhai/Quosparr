package tui

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/usage"
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

func renderDashboard(l l10n.Localizer, view View, columns, rows, page, monthOffset int, now time.Time, color bool) string {
	w := max(0, columns-1)
	if w == 0 || rows <= 0 {
		return ""
	}
	dim, bright, accent := "38;5;245", "1;38;5;255", "38;5;114"
	footer := "r " + l.Text(l10n.KeyRefresh) + "   c " + l.TUI("configure") + "   q/Esc " + l.Text(l10n.KeyQuit)
	if len(view.History) > 0 {
		footer = "[/] " + l.TUI("historyMonth") + "  0 " + l.TUI("historyThisMonth") + "   " + footer
	}
	if w < 12 || rows < 6 {
		lines := []string{fit("Quosparr", w), fit(l.TUI("resize"), w), fit("n/p r c q/Esc", w)}
		return strings.Join(lines[:min(rows, len(lines))], "\r\n")
	}
	status := l.TUI("standalone") + "  ·  " + l.TUI("auto") + " " + time.Duration(view.IntervalSeconds*int(time.Second)).String()
	if view.Refreshing {
		status = l.TUI("refreshing")
	}
	subtitle := l.TUI("overview") + " · " + l.TUI("remaining") + "  /  " + status
	if cells(subtitle) > w {
		subtitle = status
	}
	if cells(subtitle) > w && !view.Refreshing {
		subtitle = l.TUI("auto") + " " + time.Duration(view.IntervalSeconds*int(time.Second)).String()
	}
	lines := []string{
		styled(paired("Quosparr", now.Local().Format("15:04:05"), w), bright, color),
		styled(fit(subtitle, w), dim, color),
		"",
	}
	compact := w < 74 || rows < 24
	available := rows - 5
	if compact {
		lines = lines[:2]
		if rows < 12 {
			lines = lines[:1]
		}
		available = rows - len(lines) - 1
	}
	cols := 1
	if len(view.History) > 0 {
		cols = max(1, min(3, (w+2)/56))
	} else {
		if w >= 74 {
			cols = 2
		}
		if w >= 116 {
			cols = 3
		}
	}
	cardWidth := (w - (cols-1)*2) / cols
	providers := view.Floating.Providers
	var cards [][]string
	for _, p := range providers {
		card := renderCard(l, p, cardWidth, compact, color)
		sections := [][]string{card}
		if history, ok := view.History[p.Provider]; ok {
			body := renderHistory(l, history, cardWidth-4, monthOffset, now, color)
			historyCard := append([]string{card[0], card[1]}, body...)
			historyCard = append(historyCard, card[len(card)-1])
			for i := 2; i < len(historyCard)-1; i++ {
				historyCard[i] = styled("│ ", "38;5;239", color) + historyCard[i] + styled(" │", "38;5;239", color)
			}
			if len(card)+len(body) <= available {
				combined := append([]string{}, card[:len(card)-1]...)
				combined = append(combined, historyCard[2:len(historyCard)-1]...)
				sections = [][]string{append(combined, card[len(card)-1])}
			} else {
				sections = [][]string{card, historyCard}
			}
		}
		for _, section := range sections {
			for start := 2; start < len(section)-1; start += available - 3 {
				end := min(len(section)-1, start+available-3)
				fragment := append([]string{section[0], section[1]}, section[start:end]...)
				cards = append(cards, append(fragment, section[len(section)-1]))
			}
		}
	}
	pages := [][]string{{}}
	for offset := 0; offset < len(cards); offset += cols {
		group := cards[offset:min(len(cards), offset+cols)]
		height := 0
		for _, card := range group {
			height = max(height, len(card))
		}
		if len(pages[len(pages)-1])+height > available {
			pages = append(pages, nil)
		}
		for row := 0; row < height; row++ {
			parts := make([]string, len(group))
			for i, card := range group {
				parts[i] = strings.Repeat(" ", cardWidth)
				if row < len(card) {
					parts[i] = card[row]
				}
			}
			pages[len(pages)-1] = append(pages[len(pages)-1], strings.Join(parts, "  "))
		}
	}
	page = (page%len(pages) + len(pages)) % len(pages)
	lines = append(lines, pages[page]...)
	if len(providers) == 0 {
		lines = append(lines, styled(fit(l.Text(l10n.KeyNotLoaded), w), dim, color))
	}
	if compact {
		for len(lines) < rows-1 {
			lines = append(lines, "")
		}
		help := "[/] 0 n/p r c q"
		pageLabel := fmt.Sprintf("%d/%d", page+1, len(pages))
		if cells(help)+len(pageLabel)+1 > w {
			help = "[]np q"
		}
		lines = append(lines, styled(paired(help, pageLabel, w), accent, color))
		return strings.Join(lines, "\r\n")
	}
	for len(lines) < rows-2 {
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
	if len(pages) > 1 {
		pageText = fmt.Sprintf("n/p %s %d/%d", l.TUI("page"), page+1, len(pages))
	}
	if cells(footer)+cells(pageText)+1 > w {
		footer = "r c q/Esc"
		if len(view.History) > 0 {
			footer = "[/] 0 " + footer
		}
	}
	lines = append(lines, styled(paired(footer, pageText, w), accent, color))
	return strings.Join(lines[:min(rows, len(lines))], "\r\n")
}

func renderHistory(l l10n.Localizer, history TokenHistory, width, monthOffset int, now time.Time, color bool) []string {
	month := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, monthOffset, 0)
	end := month.AddDate(0, 1, -1)
	through := now.UTC().Truncate(24 * time.Hour)
	if !history.Through.IsZero() && history.Through.Before(through) {
		through = history.Through
	}
	if end.Before(through) {
		through = end
	}
	start := month.AddDate(0, 0, -int(month.Weekday()))
	weeks := int(end.Sub(start)/(7*24*time.Hour)) + 1
	weekdays := strings.Split(l.TUI("historyWeekdays"), " ")
	labelWidth := 0
	for _, label := range weekdays {
		labelWidth = max(labelWidth, cells(label))
	}
	labelWidth = max(0, min(labelWidth, width-weeks))
	prefixWidth := 0
	if labelWidth > 0 {
		prefixWidth = min(labelWidth+1, width-weeks)
	}
	cellWidth := max(1, min(4, (width-prefixWidth)/weeks))
	from := month
	if history.From.After(from) {
		from = history.From
	}
	var total int64
	var positive []int64
	recorded := 0
	for day := from; !day.After(through); day = day.AddDate(0, 0, 1) {
		value, known := history.Days[day.Format("2006-01-02")]
		if known {
			recorded++
		}
		total += value
		if value > 0 {
			positive = append(positive, value)
		}
	}
	slices.Sort(positive)
	var quartiles [3]int64
	if len(positive) > 0 {
		for i := range quartiles {
			quartiles[i] = positive[int(math.Round(float64(len(positive)-1)*float64(i+1)/4))]
		}
	}
	value := usage.CompactTokenCount(total, l.Language()) + " " + l.Text(l10n.KeyTokens)
	if !history.Available || recorded == 0 {
		value = "—"
	}
	state := l.TUI("historyLocal")
	switch {
	case history.Loading:
		state = l.TUI("refreshing")
	case history.Failed:
		state = l.TUI("historyFailed")
	case history.Incomplete:
		state = l.TUI("historyPartial")
	case !history.Available || recorded == 0:
		state = l.TUI("historyEmpty")
	}
	dateRange := month.Format("2006-01") + " UTC"
	stateInLegend := cells(dateRange)+cells(state)+1 > width
	if !stateInLegend {
		dateRange = paired(dateRange, state, width)
	}
	lines := []string{
		styled(pad(paired(l.Text(l10n.KeyTokenUsageHeatmap), value, width), width), "1;38;5;117", color),
		styled(pad(dateRange, width), "38;5;245", color),
	}
	shades := []string{"38;5;238", "38;5;24", "38;5;31", "38;5;39", "38;5;81"}
	for weekday, label := range weekdays {
		line := ""
		if labelWidth > 0 {
			line = styled(pad(label, labelWidth)+strings.Repeat(" ", prefixWidth-labelWidth), "38;5;245", color)
		}
		for week := 0; week < weeks; week++ {
			day := start.AddDate(0, 0, week*7+weekday)
			value, known := history.Days[day.Format("2006-01-02")]
			cell, shade := "·", "38;5;240"
			if day.Before(from) || day.After(through) {
				cell = " "
			} else if known {
				level := 0
				if value > 0 {
					level = 1
					for _, q := range quartiles {
						if value > q {
							level++
						}
					}
				}
				cell = string([]rune("□░▒▓█")[level])
				if color && level > 0 {
					cell = "■"
				}
				shade = shades[level]
			}
			if day.Before(month) || day.After(end) {
				line += strings.Repeat(" ", cellWidth)
				continue
			}
			if cellWidth >= 3 {
				line += styled(day.Format("02"), "38;5;245", color)
			}
			line += styled(cell, shade, color)
			padding := cellWidth - 1
			if cellWidth >= 3 {
				padding -= 2
			}
			line += strings.Repeat(" ", padding)
		}
		lines = append(lines, line+strings.Repeat(" ", max(0, width-prefixWidth-weeks*cellWidth)))
	}
	legend := "· " + l.TUI("historyMissing") + "  □ 0  " + l.TUI("historyLowHigh") + " "
	if stateInLegend {
		legend = state + "  " + legend
	}
	if color && cells(legend)+4 <= width {
		line := styled(legend, "38;5;245", color)
		for _, shade := range shades[1:] {
			line += styled("■", shade, color)
		}
		lines = append(lines, line+strings.Repeat(" ", width-cells(legend)-4))
	} else {
		lines = append(lines, styled(pad(legend+"░▒▓█", width), "38;5;245", color))
	}
	return lines
}

func renderCard(l l10n.Localizer, p runway.FloatingProvider, width int, compact, color bool) []string {
	inner := width - 4
	border := "38;5;239"
	lines := []string{styled("╭"+strings.Repeat("─", width-2)+"╮", border, color)}
	row := func(s, style string) {
		lines = append(lines, styled("│ ", border, color)+styled(pad(s, inner), style, color)+styled(" │", border, color))
	}
	if compact && cells(p.Name)+cells(p.LiveText)+1 > inner {
		row(p.Name, "1;38;5;255")
		row(p.LiveText, "1;38;5;255")
	} else {
		row(paired(p.Name, p.LiveText, inner), "1;38;5;255")
	}
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
		if compact {
			if m.Reset != "" && cells(m.Label)+cells(value)+cells(m.Reset)+4 <= inner {
				value += " · " + m.Reset
			}
			label := m.Label
			if n := min(12, inner-cells(label)-cells(value)-2); n >= 3 && m.ValueText == "" {
				filled := percent * n / 100
				label += " " + strings.Repeat("━", filled) + strings.Repeat("─", n-filled)
			}
			row(paired(label, value, inner), "1;"+shade)
			continue
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
	for len(lines) < 3 || !compact && len(lines) < 8 {
		row("", "0")
	}
	return append(lines, styled("╰"+strings.Repeat("─", width-2)+"╯", border, color))
}
