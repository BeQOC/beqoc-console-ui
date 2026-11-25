//go:build !windows

package theme_test

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/BeQOC/beqoc-console-ui/internal/ptytest"
	themeapi "github.com/BeQOC/beqoc-console-ui/theme"
)

// TestAutoProfileDetectsTrueColorOnARealTerminal proves that the automatic
// profile reads a real terminal instead of collapsing to plain text.
//
// GREEN: mutação profileFromColorProfile sempre ProfileNone (6.1).
func TestAutoProfileDetectsTrueColorOnARealTerminal(t *testing.T) {
	terminal := ptytest.OpenTerminal(t, 80, 24)
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("TTY_FORCE", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")

	consoleTheme := themeapi.New(themeapi.WithOutput(terminal.Slave()))
	if got := consoleTheme.Profile(); got != themeapi.ProfileTrueColor {
		t.Fatalf("GREEN: mutação profileFromColorProfile (6.1): auto profile on a truecolor terminal = %s, want %s",
			got, themeapi.ProfileTrueColor)
	}

	rendered := consoleTheme.Render(consoleTheme.PrimaryStyle(), "server")
	if !strings.Contains(rendered, "\x1b[38;2;") {
		t.Fatalf("render under truecolor = %q, want a 24-bit foreground", rendered)
	}

	// The same string must survive the trip through a real terminal.
	if _, err := io.WriteString(terminal.Slave(), rendered); err != nil {
		t.Fatalf("write to the terminal: %v", err)
	}
	if _, ok := terminal.Output().WaitFor("\x1b[38;2;", 5*time.Second); !ok {
		t.Fatalf("the terminal did not deliver the 24-bit sequence; output: %q", terminal.Output().String())
	}
}
