// Package tui displays the floating-window view without a desktop window.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"golang.org/x/text/width"
)

var ErrNotTerminal = errors.New("interactive terminal required")

const enterScreen = "\x1b[?1049h\x1b[?25l\x1b[0m"
const leaveScreen = "\x1b[0m\x1b[?25h\x1b[?1049l"

type terminal interface {
	read() ([]byte, error)
	size() (int, int, error)
	restore() error
}

// CheckTerminal checks both handles without changing their modes.
func CheckTerminal(in, out *os.File) error {
	if in == nil || out == nil {
		return ErrNotTerminal
	}
	return checkTerminal(in, out)
}

// Run restores the terminal on cancellation, keyboard exit, or an I/O error.
// refresh should schedule work without waiting for a network response.
func Run(ctx context.Context, in, out *os.File, l l10n.Localizer, snapshot func() runway.FloatingView, refresh func()) error {
	if err := CheckTerminal(in, out); err != nil {
		return fmt.Errorf("%s: %w", l.TUI("terminalRequired"), err)
	}
	t, err := openTerminal(in, out)
	if err != nil {
		return fmt.Errorf("%s: %w", l.TUI("terminalError"), err)
	}
	if err := run(ctx, out, t, l, snapshot, refresh); err != nil {
		return fmt.Errorf("%s: %w", l.TUI("terminalError"), err)
	}
	return nil
}

func run(ctx context.Context, out io.Writer, term terminal, l l10n.Localizer, snapshot func() runway.FloatingView, refresh func()) (err error) {
	defer func() {
		_, screenErr := io.WriteString(out, leaveScreen)
		err = errors.Join(err, screenErr, term.restore())
	}()
	if _, err = io.WriteString(out, enterScreen); err != nil {
		return err
	}
	var nextFrame time.Time
	lastWidth, lastHeight := 0, 0
	for ctx.Err() == nil {
		w, h, sizeErr := term.size()
		if sizeErr != nil {
			return sizeErr
		}
		if now := time.Now(); !now.Before(nextFrame) || w != lastWidth || h != lastHeight {
			frame := render(l, snapshot(), w, h)
			if _, err = io.WriteString(out, "\x1b[H"+frame+"\x1b[J"); err != nil {
				return err
			}
			lastWidth, lastHeight = w, h
			nextFrame = now.Add(time.Second)
		}
		input, readErr := term.read()
		if readErr != nil {
			return readErr
		}
	keys:
		for _, key := range input {
			switch key {
			case 'q', 'Q', 3:
				return nil
			case 27:
				if len(input) == 1 {
					return nil
				}
				// Escape sequences (arrows, function keys) are not quit commands.
				break keys
			case 'r', 'R':
				if refresh != nil {
					refresh()
				}
				nextFrame = time.Time{}
			}
		}
	}
	return nil
}

func render(l l10n.Localizer, view runway.FloatingView, columns, rows int) string {
	// Leave the final column free so a full line cannot wrap or scroll.
	columns = max(0, columns-1)
	if columns == 0 || rows <= 0 {
		return ""
	}
	lines := []string{"Quosparr | " + l.TUI("remaining")}
	for _, provider := range view.Providers {
		name := provider.Name
		if provider.LiveText != "" {
			name += "  " + provider.LiveText
		}
		lines = append(lines, name)
		if len(provider.Meters) == 0 {
			lines = append(lines, "  "+provider.Status)
		}
		for _, meter := range provider.Meters[:min(2, len(provider.Meters))] {
			value := meter.ValueText
			if value == "" {
				percent := min(100, max(0, meter.RemainingPercent))
				barWidth := min(20, max(0, columns-24))
				if barWidth >= 4 {
					filled := percent * barWidth / 100
					value = "[" + strings.Repeat("#", filled) + strings.Repeat("-", barWidth-filled) + "] "
				}
				value += fmt.Sprintf("%d%%", percent)
			}
			lines = append(lines, "  "+meter.Label+"  "+value)
			if meter.Reset != "" {
				lines = append(lines, "    "+l.TUI("reset")+" "+meter.Reset)
			}
		}
	}
	if len(view.Providers) == 0 {
		lines = append(lines, l.Text(l10n.KeyNotLoaded))
	}
	footer := "r " + l.Text(l10n.KeyRefresh) + " | q/Esc " + l.Text(l10n.KeyQuit)
	if len(lines)+1 > rows {
		lines = lines[:max(0, rows-1)]
		if len(lines) > 1 {
			lines[len(lines)-1] = "... " + l.TUI("resize")
		}
	}
	lines = append(lines, footer)
	for i, line := range lines {
		lines[i] = "\x1b[2K" + fit(line, columns)
	}
	return strings.Join(lines, "\r\n")
}

// fit removes terminal controls and clips by display cells, including CJK.
func fit(s string, cells int) string {
	var out strings.Builder
	used := 0
	for _, r := range s {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			continue
		}
		n := 1
		if unicode.In(r, unicode.Mn, unicode.Me) {
			if used == 0 {
				continue
			}
			n = 0
		} else if k := width.LookupRune(r).Kind(); k == width.EastAsianWide || k == width.EastAsianFullwidth || r >= 0x1f000 && r <= 0x1faff || r >= 0x2600 && r <= 0x27ff {
			n = 2
		}
		if used+n > cells {
			break
		}
		out.WriteRune(r)
		used += n
	}
	return out.String()
}
