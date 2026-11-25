//go:build !windows

package prompt_test

import (
	"strings"
	"testing"

	"github.com/BeQOC/beqoc-console-ui/internal/ptytest"
	themeapi "github.com/BeQOC/beqoc-console-ui/theme"
)

// colorfulTerminal opens a real terminal and makes the automatic profile ask
// for truecolor, so a later environment change is what disables the colors.
func colorfulTerminal(t *testing.T) *ptytest.Terminal {
	t.Helper()
	terminal := ptytest.OpenTerminal(t, 80, 24)
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("TTY_FORCE", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")

	control := themeapi.New(themeapi.WithOutput(terminal.Slave()))
	if got := control.Profile(); got != themeapi.ProfileTrueColor {
		t.Fatalf("control profile = %s, want truecolor on a real terminal", got)
	}
	return terminal
}

// assertPromptEmitsNoEscapes checks every prompt mode in line mode.
func assertPromptEmitsNoEscapes(t *testing.T, consoleTheme *themeapi.Theme) {
	t.Helper()
	for _, testCase := range accessibleCases() {
		t.Run(testCase.name, func(t *testing.T) {
			output := accessibleOutput(t, consoleTheme, testCase)
			// RED: Relatório P0-5 (C2 corrige).
			if strings.ContainsRune(output, '\x1b') {
				t.Errorf("RED: Relatório P0-5 (C2 corrige): prompt output = %q, want no escape sequences", output)
			}
		})
	}
}

// TestPromptWithNoColorEmitsNoEscapes protects P0-5 for NO_COLOR: the variable
// must reach the prompt styles, not just the theme helpers.
//
// The owner's decision about whether NO_COLOR keeps text attributes is still
// open (relatório, decisão 3). This test only encodes today's documented
// contract: NO_COLOR renders ProfileNone, and ProfileNone emits nothing.
func TestPromptWithNoColorEmitsNoEscapes(t *testing.T) {
	terminal := colorfulTerminal(t)

	t.Setenv("NO_COLOR", "1")
	plain := themeapi.New(themeapi.WithOutput(terminal.Slave()))
	if got := plain.Profile(); got != themeapi.ProfileNone {
		t.Fatalf("NO_COLOR profile = %s, want none", got)
	}

	assertPromptEmitsNoEscapes(t, plain)
}

// TestPromptWithDumbTerminalEmitsNoEscapes protects P0-5 for TERM=dumb, which
// reaches the prompt through the same automatic profile.
func TestPromptWithDumbTerminalEmitsNoEscapes(t *testing.T) {
	terminal := colorfulTerminal(t)

	t.Setenv("TERM", "dumb")
	plain := themeapi.New(themeapi.WithOutput(terminal.Slave()))
	if got := plain.Profile(); got != themeapi.ProfileNone {
		t.Fatalf("TERM=dumb profile = %s, want none", got)
	}

	assertPromptEmitsNoEscapes(t, plain)
}
