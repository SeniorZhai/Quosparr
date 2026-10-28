package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/platform"
)

const HelpText = `Quosparr TUI

Usage: quosparr [--tui | --setup | --help]

  --tui    Open the independent quota dashboard (default).
  --setup  Configure providers, credentials, language and automatic refresh.
  --help   Show this help.

Keys: r refresh · c configure · n/p or Tab browse · q/Esc quit
`

// Command runs either executable's terminal entry without loading the App.
func Command(args []string) (code int) {
	setup := false
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			fmt.Print(HelpText)
			return 0
		case "--setup":
			setup = true
		case "--tui":
		default:
			fmt.Fprint(os.Stderr, HelpText)
			return 1
		}
	}
	l := l10n.ForPreference("system", l10n.SystemPreferredLanguages())
	restore, err := PrepareConsole()
	if err != nil {
		fmt.Fprintln(os.Stderr, l.TUI("terminalRequired"))
		return 1
	}
	defer func() {
		if err := restore(); err != nil {
			fmt.Fprintln(os.Stderr, l.TUI("terminalError"))
			code = 1
		}
	}()
	if err := CheckTerminal(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, l.TUI("terminalRequired"))
		return 1
	}
	home, err := platform.Home()
	if err != nil {
		fmt.Fprintln(os.Stderr, l.TUI("configError"))
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	config, err := LoadConfig(home)
	if errors.Is(err, os.ErrNotExist) {
		setup = true
	} else if err != nil && !setup {
		fmt.Fprintln(os.Stderr, l.TUI("configError"))
		return 1
	}
	for ctx.Err() == nil {
		if setup {
			next, err := Configure(ctx, os.Stdin, os.Stdout, home, config)
			if errors.Is(err, ErrSetupCanceled) {
				if len(config.Providers) == 0 {
					return 0
				}
			} else if err != nil {
				fmt.Fprintln(os.Stderr, l.TUI("configError"))
				return 1
			} else {
				config = next
			}
			setup = false
		}
		if ctx.Err() != nil {
			return 0
		}
		l = l10n.New(config.Language)
		runtime := Start(ctx, config, home)
		err := Run(ctx, os.Stdin, os.Stdout, l, runtime.View, runtime.Refresh)
		runtime.Close()
		if errors.Is(err, ErrConfigure) {
			setup = true
			continue
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	return 0
}
