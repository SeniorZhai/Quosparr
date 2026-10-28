// Quosparr TUI is an independent terminal quota monitor.
package main

import (
	"github.com/licoy/quosparr/internal/tui"
	"os"
)

func main() { os.Exit(tui.Command(os.Args[1:])) }
