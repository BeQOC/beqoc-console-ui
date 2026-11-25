//go:build !windows

package consoleui_test

import (
	"bytes"
	"testing"

	"github.com/BeQOC/beqoc-console-ui"
	"github.com/BeQOC/beqoc-console-ui/internal/ptytest"
)

// TestOutputDrivesProfileDetection protects the WithOutput re-export: the
// facade must hand the writer to the theme, which is the only way to make
// automatic profile detection deterministic.
//
// GREEN: mutação "zerar as opções na raiz" (6.1).
func TestOutputDrivesProfileDetection(t *testing.T) {
	terminal := ptytest.OpenTerminal(t, 80, 24)
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("TTY_FORCE", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")

	onTerminal := consoleui.New(consoleui.WithOutput(terminal.Slave()))
	if got := onTerminal.Profile(); got != consoleui.ProfileTrueColor {
		t.Fatalf("New(WithOutput(terminal)).Profile() = %s, want truecolor", got)
	}

	onPipe := consoleui.New(consoleui.WithOutput(&bytes.Buffer{}))
	if got := onPipe.Profile(); got != consoleui.ProfileNone {
		t.Fatalf("New(WithOutput(buffer)).Profile() = %s, want none", got)
	}
}
