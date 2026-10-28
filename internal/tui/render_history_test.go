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
)

func TestHistoryDayAlignmentAndMissingRecords(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	history := TokenHistory{From: now.AddDate(0, 0, -10).Truncate(24 * time.Hour), Through: now.Truncate(24 * time.Hour), Available: true,
		Days: map[string]int64{"2026-10-18": 0, "2026-10-19": 10, "2026-10-20": 20, "2026-10-21": 30,
			"2026-10-22": 40, "2026-10-23": 50, "2026-10-24": 60, "2026-10-26": 70, "2026-10-27": 80, "2026-10-28": 90}}
	lines := renderHistory(l10n.New(l10n.English), history, 8, 0, now, false)
	want := []string{"Su    □·", "Mo    ░▓", "Tu    ░█", "We    ░█", "Th    ▒ ", "Fr    ▒ ", "Sa    ▓ "}
	for day, row := range want {
		if lines[day+2] != row {
			t.Errorf("day %d = %q, want %q", day, lines[day+2], row)
		}
	}
	colored := strings.Join(renderHistory(l10n.New(l10n.English), history, 8, 0, now, true), "\n")
	for _, shade := range []string{"38;5;24", "38;5;31", "38;5;39", "38;5;81"} {
		if !strings.Contains(colored, "\x1b["+shade+"m■") {
			t.Errorf("missing heatmap intensity %s", shade)
		}
	}
}

func TestHistoryVisibleRangeAndEmptyStates(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	history := TokenHistory{From: now.AddDate(0, 0, -370).Truncate(24 * time.Hour), Through: now.Truncate(24 * time.Hour), Available: true, Days: make(map[string]int64)}
	for day := history.From; !day.After(history.Through); day = day.AddDate(0, 0, 1) {
		history.Days[day.Format(time.DateOnly)] = 100
	}
	l := l10n.New(l10n.English)
	wide := renderHistory(l, history, 100, 0, now, false)
	if strings.Count(wide[2], "░") != 4 || !strings.Contains(wide[1], "2026-10 UTC") || !strings.Contains(wide[0], "2.8K Tokens") {
		t.Fatalf("current month range mismatch: %q", wide)
	}
	narrow := renderHistory(l, history, 23, 0, now, false)
	if strings.Count(narrow[2], "░") != 4 || !strings.Contains(narrow[0], "2.8K Tokens") {
		t.Fatalf("narrow view must keep the same full month: %q", narrow)
	}
	previous := renderHistory(l, history, 75, -1, now, false)
	if !strings.Contains(previous[1], "2026-09 UTC") || !strings.Contains(previous[0], "3K Tokens") {
		t.Fatalf("previous month range mismatch: %q", previous)
	}
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
		got := strings.Join(renderHistory(l, tc.history, 75, 0, now, false), "\n")
		if !strings.Contains(got, tc.state) || strings.Contains(got, "0 Tokens") || strings.Contains(got, "□") && strings.Count(got, "□") != 1 {
			t.Errorf("unavailable history must not claim zero usage: %q", got)
		}
	}
}

func TestHistoryLeapMonthAndSixWeekMonth(t *testing.T) {
	l := l10n.New(l10n.English)
	for _, tc := range []struct {
		month time.Time
		days  int
	}{
		{time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), 29},
		{time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), 31},
	} {
		history := TokenHistory{From: tc.month, Through: tc.month.AddDate(0, 1, 0), Available: true, Days: make(map[string]int64)}
		for day := 0; day < tc.days; day++ {
			history.Days[tc.month.AddDate(0, 0, day).Format(time.DateOnly)] = 1
		}
		history.Days[tc.month.AddDate(0, 1, 0).Format(time.DateOnly)] = 999
		for _, width := range []int{8, 23, 75} {
			lines := renderHistory(l, history, width, -1, history.Through, false)
			grid := strings.Join(lines[2:9], "\n")
			if strings.Count(grid, "░") != tc.days {
				t.Fatalf("%s width %d lost days: %q", tc.month.Format("2006-01"), width, grid)
			}
			if width >= 23 && !strings.Contains(lines[0], fmt.Sprintf("%d Tokens", tc.days)) {
				t.Fatalf("adjacent month leaked into total: %q", lines[0])
			}
			for _, line := range lines {
				if cells(line) != width {
					t.Fatalf("width %d row has %d cells: %q", width, cells(line), line)
				}
			}
		}
		for _, lang := range l10n.Languages() {
			lc := l10n.New(lang)
			lines := renderHistory(lc, history, 8, -1, history.Through, false)
			for day, label := range strings.Split(lc.TUI("historyWeekdays"), " ") {
				if !strings.HasPrefix(lines[day+2], label) {
					t.Errorf("%s lost weekday %s in a narrow month: %q", lang, label, lines[day+2])
				}
			}
		}
	}
}

func TestHistoryDashboardFitsAndPaginates(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	view := historyRenderFixture(now)
	l := l10n.New(l10n.English)
	first := renderDashboard(l, view, 80, 24, 0, 0, now, false)
	for _, want := range []string{"73%", "28%", "Token usage", "Su ", "Mo ", "Tu ", "We ", "Th ", "Fr ", "Sa ", "Page 1/3"} {
		if !strings.Contains(first, want) {
			t.Errorf("80x24 must show a full quota/heatmap card; missing %q", want)
		}
	}
	second := renderDashboard(l, view, 80, 24, 1, 0, now, false)
	third := renderDashboard(l, view, 80, 24, -1, 0, now, false)
	if !strings.Contains(second, "Claude") || strings.Contains(second, "Codex") || !strings.Contains(third, "Grok") {
		t.Fatal("provider cards must remain reachable in both page directions")
	}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, size := range [][2]int{{80, 24}, {126, 24}, {192, 44}, {60, 18}, {40, 15}, {20, 8}, {13, 6}} {
		for _, lang := range l10n.Languages() {
			var allPages strings.Builder
			for page := 0; page < 48; page++ {
				frame := renderDashboard(l10n.New(lang), view, size[0], size[1], page, 0, now, true)
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
		lines := strings.Split(renderDashboard(l, view, 40, 18, 0, 0, now, false), "\r\n")
		if !strings.Contains(lines[1], l.TUI("auto")+" 5m0s") {
			t.Errorf("%s: refresh interval was clipped: %q", language, lines[1])
		}
		view.Refreshing = true
		lines = strings.Split(renderDashboard(l, view, 40, 18, 0, 0, now, false), "\r\n")
		if !strings.Contains(lines[1], l.TUI("refreshing")) {
			t.Errorf("%s: refresh state was clipped: %q", language, lines[1])
		}
	}
}

func TestCompactDashboardKeepsQuotaAndMonthOnOnePage(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	view := historyRenderFixture(now)
	for _, size := range [][2]int{{40, 18}, {60, 18}, {80, 18}} {
		for _, lang := range l10n.Languages() {
			lc := l10n.New(lang)
			frame := renderDashboard(lc, view, size[0], size[1], 0, 0, now, false)
			for _, want := range []string{"Codex", "73%", "28%", "2026-10 UTC", "[/] 0", "n/p", "q"} {
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
			frame := renderDashboard(l10n.New(l10n.English), historyRenderFixture(now), size[0], size[1], page, 0, now, false)
			lines := strings.Split(frame, "\r\n")
			footer := lines[len(lines)-1]
			if !strings.Contains(footer, "q") || !strings.Contains(footer, "np") || !strings.Contains(footer, "[]") {
				t.Errorf("%v page %d lost controls: %q", size, page, footer)
			}
		}
	}
}

func historyRenderFixture(now time.Time) View {
	history := TokenHistory{From: now.AddDate(0, 0, -370).Truncate(24 * time.Hour), Through: now.Truncate(24 * time.Hour), Available: true, Days: make(map[string]int64)}
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
					frame := renderDashboard(l10n.New(lang), historyRenderFixture(now), size[0], size[1], page, 0, now, color)
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
