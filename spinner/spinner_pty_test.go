//go:build !windows

package spinner_test

import (
	"strings"
	"testing"
	"time"

	"github.com/BeQOC/beqoc-console-ui/internal/ptytest"
	"github.com/BeQOC/beqoc-console-ui/spinner"
)

const ptyWait = 5 * time.Second

func TestSpinnerAnimatesOnATerminal(t *testing.T) {
	terminal := ptytest.OpenTerminal(t, 80, 24)
	t.Setenv("TERM", "xterm-256color")

	progress := spinner.New("Resolving dependencies",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(terminal.Slave()),
		spinner.WithInterval(5*time.Millisecond),
	).Start()

	frames := spinner.DefaultFrames()
	output := terminal.Output()
	if _, ok := output.WaitForFunc(func(data string) bool {
		if strings.Count(data, "\r\x1b[2K") < 4 {
			return false
		}
		seen := 0
		for _, frame := range frames {
			if strings.Contains(data, frame+" Resolving dependencies") {
				seen++
			}
		}
		return seen >= 2
	}, ptyWait); !ok {
		t.Fatalf("the spinner did not animate; output: %q", output.String())
	}

	progress.Succeed("done")
	if _, ok := output.WaitFor("✔ done", ptyWait); !ok {
		t.Fatalf("the spinner did not print its final message; output: %q", output.String())
	}

	got := output.String()
	if !strings.Contains(got, "\r\x1b[2K✔ done") {
		t.Fatalf("the final message does not replace the frame: %q", got)
	}
	if tail := strings.TrimRight(got, "\r\n"); strings.HasSuffix(tail, "Resolving dependencies") {
		t.Fatalf("the last frame was left on screen instead of being replaced: %q", got)
	}
}

func TestSpinnerStaysPlainOnADumbTerminal(t *testing.T) {
	terminal := ptytest.OpenTerminal(t, 80, 24)
	t.Setenv("TERM", "dumb")

	progress := spinner.New("Resolving dependencies",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(terminal.Slave()),
		spinner.WithInterval(5*time.Millisecond),
	).Start()
	progress.Update("still working")
	progress.Succeed("done")

	output := terminal.Output()
	if _, ok := output.WaitFor("✔ done", ptyWait); !ok {
		t.Fatalf("the spinner did not print its final message; output: %q", output.String())
	}

	got := output.String()
	if strings.Contains(got, "\x1b[2K") {
		t.Fatalf("a dumb terminal received a redraw sequence: %q", got)
	}
	if trimmed := strings.TrimSpace(got); trimmed != "✔ done" {
		t.Fatalf("output = %q, want only the final message on a dumb terminal", got)
	}
}
