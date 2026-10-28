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
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
	"golang.org/x/text/width"
)

var ErrNotTerminal = errors.New("interactive terminal required")
var ErrConfigure = errors.New("configure TUI")

type View struct {
	Floating        runway.FloatingView
	History         map[store.Provider]TokenHistory
	SourceAt        time.Time
	LastRefresh     time.Time
	NextRefresh     time.Time
	IntervalSeconds int
	Refreshing      bool
}

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
func Run(ctx context.Context, in, out *os.File, l l10n.Localizer, snapshot func() View, refresh func()) error {
	if err := CheckTerminal(in, out); err != nil {
		return fmt.Errorf("%s: %w", l.TUI("terminalRequired"), err)
	}
	t, err := openTerminal(in, out)
	if err != nil {
		return fmt.Errorf("%s: %w", l.TUI("terminalError"), err)
	}
	if err := run(ctx, out, t, l, snapshot, refresh); err != nil {
		if errors.Is(err, ErrConfigure) {
			return err
		}
		return fmt.Errorf("%s: %w", l.TUI("terminalError"), err)
	}
	return nil
}

func run(ctx context.Context, out io.Writer, term terminal, l l10n.Localizer, snapshot func() View, refresh func()) (err error) {
	defer func() {
		_, screenErr := io.WriteString(out, leaveScreen)
		err = errors.Join(err, screenErr, term.restore())
	}()
	if _, err = io.WriteString(out, enterScreen); err != nil {
		return err
	}
	var nextFrame time.Time
	lastWidth, lastHeight := 0, 0
	page, historyOffset := 0, 0
	mode := usage.HeatmapDaily
	color := os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	for ctx.Err() == nil {
		w, h, sizeErr := term.size()
		if sizeErr != nil {
			return sizeErr
		}
		if now := time.Now(); !now.Before(nextFrame) || w != lastWidth || h != lastHeight {
			historyOffset = max(historyScrollLimit(l, w, now), min(0, historyOffset))
			frame := renderDashboard(l, snapshot(), w, h, page, historyOffset, mode, time.Now(), color)
			prefix := "\x1b[H"
			if color {
				prefix += "\x1b[48;5;234m\x1b[38;5;252m"
			}
			if _, err = io.WriteString(out, prefix+"\x1b[2K"+strings.ReplaceAll(frame, "\r\n", "\r\n\x1b[2K")+"\x1b[J"); err != nil {
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
			case 'c', 'C':
				return ErrConfigure
			case 'n', 'N', '\t':
				page++
				nextFrame = time.Time{}
			case 'p', 'P':
				page--
				nextFrame = time.Time{}
			case '[':
				historyOffset = max(historyScrollLimit(l, w, time.Now()), historyOffset-4)
				nextFrame = time.Time{}
			case ']':
				historyOffset = min(0, historyOffset+4)
				nextFrame = time.Time{}
			case '0':
				historyOffset = 0
				nextFrame = time.Time{}
			case '1':
				mode = usage.HeatmapDaily
				nextFrame = time.Time{}
			case '2':
				mode = usage.HeatmapWeekly
				nextFrame = time.Time{}
			case '3':
				mode = usage.HeatmapCumulative
				nextFrame = time.Time{}
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
