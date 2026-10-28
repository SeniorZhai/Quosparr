package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestHistoryModesMatchAppMetricsAndMissingRecords(t *testing.T) {
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	history := TokenHistory{Available: true, Days: map[string]int64{
		"2025-12-31": 999, "2026-01-01": 10, "2026-01-02": 20,
		"2026-01-05": 0, "2026-01-06": 30, "2026-01-11": 999,
	}}
	l := l10n.New(l10n.English)
	for _, tc := range []struct {
		mode                    usage.HeatmapMode
		label                   string
		positive, missing, zero int
	}{
		{usage.HeatmapDaily, "[1 Daily]", 3, 6, 1},
		{usage.HeatmapWeekly, "[2 Weekly]", 10, 0, 0},
		{usage.HeatmapCumulative, "[3 Total]", 10, 0, 0},
	} {
		lines := renderHistory(l, history, 75, 0, tc.mode, now, false)
		grid := strings.Join(lines[2:9], "")
		positive := strings.Count(grid, "░") + strings.Count(grid, "▒") + strings.Count(grid, "▓") + strings.Count(grid, "█")
		if !strings.Contains(lines[0], "60 Tokens") || !strings.Contains(lines[1], tc.label) || positive != tc.positive || strings.Count(grid, "·") != tc.missing || strings.Count(grid, "□") != tc.zero {
			t.Fatalf("%s: mismatched mode/known records: %q", tc.mode, lines)
		}
		if tc.mode == usage.HeatmapWeekly && strings.Count(grid, "░") != 10 {
			t.Fatalf("both weeks total 30, so all their daily cells must share an intensity: %q", grid)
		}
		if tc.mode == usage.HeatmapCumulative && !strings.Contains(lines[2], "░") {
			t.Fatal("the cumulative total must carry through January 4 even with no record that day")
		}
	}
	daily := renderHistory(l, history, 75, 0, usage.HeatmapDaily, now, false)
	if !strings.HasPrefix(daily[3], "Mo  □") || !strings.HasPrefix(daily[6], "Th ░·") {
		t.Fatalf("Sunday-first day alignment changed: %q", daily)
	}
}

func TestHistoryYearRangeMonthLabelsAndHorizontalScrolling(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	history := TokenHistory{From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Through: now.Truncate(24 * time.Hour), Available: true, Days: make(map[string]int64)}
	for day := history.From; !day.After(history.Through); day = day.AddDate(0, 0, 1) {
		history.Days[day.Format(time.DateOnly)] = 100
	}
	l := l10n.New(l10n.English)
	wide := renderHistory(l, history, 75, 0, usage.HeatmapDaily, now, false)
	if !strings.Contains(wide[0], "30.1K Tokens") || !strings.Contains(wide[1], "2026 UTC") || strings.Count(wide[2], "░") != 43 {
		t.Fatalf("year-to-date range mismatch: %q", wide)
	}
	for month := 1; month <= 10; month++ {
		if !strings.Contains(wide[9], runway.MonthTitle(month, l.Language())) {
			t.Errorf("missing month %d in %q", month, wide[9])
		}
	}
	latest := renderHistory(l, history, 35, 0, usage.HeatmapDaily, now, false)
	previous := renderHistory(l, history, 35, -4, usage.HeatmapDaily, now, false)
	oldest := renderHistory(l, history, 35, -500, usage.HeatmapDaily, now, false)
	limit := historyScrollLimit(l, 40, now)
	clamped := renderHistory(l, history, 35, limit, usage.HeatmapDaily, now, false)
	if limit != -12 || strings.Join(oldest, "") != strings.Join(clamped, "") || !strings.Contains(oldest[9], "Jan") || strings.Contains(latest[9], "Jan") || latest[9] == previous[9] {
		t.Fatalf("viewport boundaries: limit=%d, latest=%q previous=%q oldest=%q", limit, latest[9], previous[9], oldest[9])
	}
	for _, lines := range [][]string{latest, previous, oldest} {
		if !strings.Contains(lines[0], "30.1K Tokens") {
			t.Fatal("scrolling must not change the full-year total")
		}
	}
}

func TestHistoryEmptyStatesLeapDaysAndNarrowModes(t *testing.T) {
	now := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
	l := l10n.New(l10n.English)
	for _, tc := range []struct {
		history TokenHistory
		state   string
	}{
		{TokenHistory{}, "No local records"},
		{TokenHistory{Loading: true}, "Refreshing"},
		{TokenHistory{Failed: true}, "Scan failed"},
		{TokenHistory{Incomplete: true}, "Incomplete records"},
		{TokenHistory{Available: true, Days: map[string]int64{"2020-01-01": 100}}, "No local records"},
	} {
		got := strings.Join(renderHistory(l, tc.history, 75, 0, usage.HeatmapDaily, now, false), "\n")
		if !strings.Contains(got, tc.state) || strings.Contains(got, "0 Tokens") || strings.Contains(got, "□") {
			t.Errorf("unavailable history must not claim zero usage: %q", got)
		}
	}
	history := TokenHistory{Available: true, Days: map[string]int64{"2024-01-01": 1, "2024-02-29": 1, "2024-03-02": 999}}
	wide := renderHistory(l, history, 75, 0, usage.HeatmapDaily, now, false)
	if !strings.Contains(wide[0], "2 Tokens") || strings.Count(strings.Join(wide[2:9], ""), "░") != 2 {
		t.Fatalf("leap day was omitted or future usage included: %q", wide)
	}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, lang := range l10n.Languages() {
		lc := l10n.New(lang)
		for _, mode := range []usage.HeatmapMode{usage.HeatmapDaily, usage.HeatmapWeekly, usage.HeatmapCumulative} {
			for _, width := range []int{8, 15, 35, 75} {
				lines := renderHistory(lc, history, width, -4, mode, now, true)
				if len(lines) != 10 {
					t.Fatalf("history must fit ten rows: %d", len(lines))
				}
				for i, line := range lines {
					line = ansi.ReplaceAllString(line, "")
					if cells(line) != width {
						t.Fatalf("%s %s width=%d row=%d has %d cells: %q", lang, mode, width, i, cells(line), line)
					}
					if i >= 2 && i < 9 && !strings.HasPrefix(line, strings.Split(lc.TUI("historyWeekdays"), " ")[i-2]) {
						t.Errorf("%s lost its weekday label: %q", lang, line)
					}
				}
			}
		}
	}
}

func TestHistoryDashboardFitsAndPaginates(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	view := historyRenderFixture(now)
	l := l10n.New(l10n.English)
	first := renderDashboard(l, view, 80, 24, 0, 0, usage.HeatmapDaily, now, false)
	for _, want := range []string{"73%", "28%", "Token usage", "Su ", "Mo ", "Tu ", "We ", "Th ", "Fr ", "Sa ", "Page 1/3"} {
		if !strings.Contains(first, want) {
			t.Errorf("80x24 must show a full quota/heatmap card; missing %q", want)
		}
	}
	second := renderDashboard(l, view, 80, 24, 1, 0, usage.HeatmapDaily, now, false)
	third := renderDashboard(l, view, 80, 24, -1, 0, usage.HeatmapDaily, now, false)
	if !strings.Contains(second, "Claude") || strings.Contains(second, "Codex") || !strings.Contains(third, "Grok") {
		t.Fatal("provider cards must remain reachable in both page directions")
	}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, size := range [][2]int{{80, 24}, {126, 24}, {192, 44}, {60, 18}, {40, 15}, {20, 8}, {13, 6}} {
		for _, lang := range l10n.Languages() {
			var allPages strings.Builder
			for page := 0; page < 48; page++ {
				frame := renderDashboard(l10n.New(lang), view, size[0], size[1], page, 0, usage.HeatmapDaily, now, true)
				allPages.WriteString(ansi.ReplaceAllString(frame, ""))
				lines := strings.Split(frame, "\r\n")
				if len(lines) > size[1] {
					t.Fatalf("%s %v: %d rows overflow", lang, size, len(lines))
				}
				for _, line := range lines {
					line = ansi.ReplaceAllString(line, "")
					if fit(line, size[0]-1) != line {
						t.Fatalf("%s %v row overflows: %q", lang, size, line)
					}
				}
			}
			for _, weekday := range strings.Split(l10n.New(lang).TUI("historyWeekdays"), " ") {
				if !strings.Contains(allPages.String(), "│ "+weekday+" ") {
					t.Errorf("%s %v: weekday %q is unreachable", lang, size, weekday)
				}
			}
		}
	}
}

func TestNarrowDashboardKeepsRefreshSubtitle(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, language := range l10n.Languages() {
		l := l10n.New(language)
		view := historyRenderFixture(now)
		lines := strings.Split(renderDashboard(l, view, 40, 18, 0, 0, usage.HeatmapDaily, now, false), "\r\n")
		if !strings.Contains(lines[1], l.TUI("auto")+" 5m0s") {
			t.Errorf("%s: refresh interval was clipped: %q", language, lines[1])
		}
		view.Refreshing = true
		lines = strings.Split(renderDashboard(l, view, 40, 18, 0, 0, usage.HeatmapDaily, now, false), "\r\n")
		if !strings.Contains(lines[1], l.TUI("refreshing")) {
			t.Errorf("%s: refresh state was clipped: %q", language, lines[1])
		}
	}
}

func TestCompactDashboardKeepsQuotaAndYearHeatmapOnOnePage(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	view := historyRenderFixture(now)
	for _, size := range [][2]int{{40, 18}, {60, 18}, {80, 18}} {
		for _, lang := range l10n.Languages() {
			lc := l10n.New(lang)
			frame := renderDashboard(lc, view, size[0], size[1], 0, 0, usage.HeatmapDaily, now, false)
			for _, want := range []string{"Codex", "73%", "28%", "2026 UTC", "[/] 0", "n/p", "q"} {
				if !strings.Contains(frame, want) {
					t.Errorf("%s %v first page lost %q", lang, size, want)
				}
			}
			for _, weekday := range strings.Split(lc.TUI("historyWeekdays"), " ") {
				if !strings.Contains(frame, "│ "+weekday+" ") {
					t.Errorf("%s %v month was split: missing %s", lang, size, weekday)
				}
			}
		}
	}
}

func TestTinyDashboardKeepsQuitAndPageControls(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, size := range [][2]int{{20, 8}, {13, 6}} {
		for page := 0; page < 48; page++ {
			frame := renderDashboard(l10n.New(l10n.English), historyRenderFixture(now), size[0], size[1], page, 0, usage.HeatmapDaily, now, false)
			lines := strings.Split(frame, "\r\n")
			footer := lines[len(lines)-1]
			if !strings.Contains(footer, "q") || !strings.Contains(footer, "np") || !strings.Contains(footer, "[]") {
				t.Errorf("%v page %d lost controls: %q", size, page, footer)
			}
		}
	}
}

func historyRenderFixture(now time.Time) View {
	history := TokenHistory{From: time.Date(now.UTC().Year(), 1, 1, 0, 0, 0, 0, time.UTC), Through: now.Truncate(24 * time.Hour), Available: true, Days: make(map[string]int64)}
	for day, i := history.From, 0; !day.After(history.Through); day, i = day.AddDate(0, 0, 1), i+1 {
		if i%13 == 0 {
			continue
		}
		history.Days[day.Format(time.DateOnly)] = int64(i%17) * int64((i*37)%241) * 100
	}
	return View{IntervalSeconds: 300, LastRefresh: now.Add(-time.Minute), NextRefresh: now.Add(47 * time.Second),
		History: map[store.Provider]TokenHistory{store.ProviderCodex: history, store.ProviderClaude: history, store.ProviderGrok: {}},
		Floating: runway.FloatingView{Providers: []runway.FloatingProvider{
			{Provider: store.ProviderCodex, Name: "Codex", LiveText: "42 tok/s", Meters: []runway.FloatingMeter{{Label: "5h", RemainingPercent: 73, Reset: "2h 18m"}, {Label: "Weekly", RemainingPercent: 28, Reset: "3d 7h"}}},
			{Provider: store.ProviderClaude, Name: "Claude", LiveText: "16 tok/s", Meters: []runway.FloatingMeter{{Label: "5h", RemainingPercent: 91, Reset: "4h 32m"}, {Label: "Weekly", RemainingPercent: 62, Reset: "5d 14h"}}},
			{Provider: store.ProviderGrok, Name: "Grok", Status: "No local records"},
		}}}
}

func TestExportHistoryPreview(t *testing.T) {
	dir := os.Getenv("QUOSPARR_TUI_QA_DIR")
	if dir == "" {
		t.Skip("set QUOSPARR_TUI_QA_DIR to export deterministic terminal previews")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, size := range [][2]int{{80, 24}, {120, 30}, {40, 18}, {126, 24}, {60, 18}, {20, 8}} {
		for _, lang := range []l10n.Language{l10n.English, l10n.SimplifiedChinese} {
			for _, color := range []bool{false, true} {
				for page := 0; page < 2; page++ {
					frame := renderDashboard(l10n.New(lang), historyRenderFixture(now), size[0], size[1], page, 0, usage.HeatmapDaily, now, color)
					ext := "txt"
					if color {
						ext = "ansi"
						frame = "\x1b[48;5;234m\x1b[38;5;252m" + frame + "\x1b[0m"
					}
					name := fmt.Sprintf("heatmap-%s-page%d-%dx%d.%s", lang, page+1, size[0], size[1], ext)
					if err := os.WriteFile(filepath.Join(dir, name), []byte(frame+"\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
}
