//go:build !windows

package spinner_test

import (
	"errors"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/BeQOC/beqoc-console-ui/internal/ptytest"
	"github.com/BeQOC/beqoc-console-ui/spinner"
)

// TestPTYHelperProcess is the child entry point of the ptytest harness.
func TestPTYHelperProcess(t *testing.T) {
	ptytest.HelperProcess(t)
}

const (
	// narrowColumns is the width the publication report measured the spinner
	// on, where the line wrapped into four physical lines.
	narrowColumns = 20
	// wideColumns leaves room for the long line, which proves the spinner uses
	// the terminal it has instead of a fixed limit.
	wideColumns = 60
	// redrawPrefix is what the spinner writes before every frame.
	redrawPrefix = "\r\x1b[2K"
)

const (
	// overlongSpinnerText is far wider than a 20-column terminal.
	overlongSpinnerText = "Resolving dependencies with a very long line and no room to breathe"
	// fittingSpinnerText fits wideColumns and overflows narrowColumns, so a
	// correct truncation keeps it intact on a wide terminal.
	fittingSpinnerText = "Resolving the dependency tree of the tool"
)

// spinnerANSI matches the escape sequences a terminal consumes.
var spinnerANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// visibleText strips escape sequences from captured terminal output.
func visibleText(output string) string {
	return spinnerANSI.ReplaceAllString(output, "")
}

// redrawWidths reports how many cells every frame the spinner drew occupies.
// The texts this suite draws are ASCII or narrow accented runes, so one rune is
// one cell; wide CJK text is out of scope of these assertions.
func redrawWidths(output string) []int {
	var widths []int
	for _, chunk := range strings.Split(output, "\r") {
		line := strings.TrimRight(chunk, "\n")
		if line == "" {
			continue
		}
		widths = append(widths, utf8.RuneCountInString(visibleText(line)))
	}
	return widths
}

// widestRedraw reports the widest frame the spinner drew.
func widestRedraw(output string) int {
	widest := 0
	for _, width := range redrawWidths(output) {
		if width > widest {
			widest = width
		}
	}
	return widest
}

// wrappedLines counts the physical lines a redraw overflowed into. A terminal
// wraps at its last column and "\r\x1b[2K" only erases the last physical line,
// so every extra line stays on screen as a ghost line.
func wrappedLines(output string, cols int) int {
	total := 0
	for _, width := range redrawWidths(output) {
		if width > cols {
			total += (width - 1) / cols
		}
	}
	return total
}

// TestSpinnerTruncatesToTheTerminalWidth protects P1-1: a spinner line must fit
// the terminal it draws on. Today the line is written in full, wraps into four
// physical lines on a 20-column terminal and the next erase leaves the first
// three behind.
func TestSpinnerTruncatesToTheTerminalWidth(t *testing.T) {
	terminal := ptytest.OpenTerminal(t, narrowColumns, 24)
	t.Setenv("TERM", "xterm-256color")

	progress := spinner.New(overlongSpinnerText,
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(terminal.Slave()),
		spinner.WithInterval(5*time.Millisecond),
	).Start()

	output := terminal.Output()
	if _, ok := output.WaitForFunc(func(data string) bool {
		return strings.Count(data, redrawPrefix) >= 4
	}, ptyWait); !ok {
		t.Fatalf("the spinner did not animate; output: %q", output.String())
	}
	progress.Succeed("done")
	if _, ok := output.WaitFor("✔ done", ptyWait); !ok {
		t.Fatalf("the spinner did not print its final message; output: %q", output.String())
	}

	captured := output.String()
	// RED: Relatório P1-1 (C4 corrige). C4 truncates each draw to the current
	// visual width; WithWidth arrives with C4 for a deterministic override.
	if widest := widestRedraw(captured); widest > narrowColumns {
		t.Errorf("RED: Relatório P1-1 (C4 corrige): the spinner drew %d cells on a %d-column terminal",
			widest, narrowColumns)
	}
	if ghosts := wrappedLines(captured, narrowColumns); ghosts > 0 {
		t.Errorf("RED: Relatório P1-1 (C4 corrige): %d wrapped lines were left behind by the erase: %q",
			ghosts, captured)
	}
}

// TestSpinnerKeepsALineThatFitsTheTerminal is green today and keeps the fix
// honest: truncating everything to the report's 20 columns would pass the
// narrow test and silently cut a line that fits a wide terminal.
func TestSpinnerKeepsALineThatFitsTheTerminal(t *testing.T) {
	terminal := ptytest.OpenTerminal(t, wideColumns, 24)
	t.Setenv("TERM", "xterm-256color")

	progress := spinner.New(fittingSpinnerText,
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(terminal.Slave()),
		spinner.WithInterval(5*time.Millisecond),
	).Start()

	output := terminal.Output()
	if _, ok := output.WaitForFunc(func(data string) bool {
		return strings.Count(data, redrawPrefix) >= 2
	}, ptyWait); !ok {
		t.Fatalf("the spinner did not animate; output: %q", output.String())
	}
	progress.Succeed("done")
	if _, ok := output.WaitFor("✔ done", ptyWait); !ok {
		t.Fatalf("the spinner did not print its final message; output: %q", output.String())
	}

	if captured := visibleText(output.String()); !strings.Contains(captured, fittingSpinnerText) {
		t.Errorf("the spinner truncated a line that fits %d columns: %q", wideColumns, captured)
	}
}

func init() {
	// spinner-resize draws a long line until the parent resizes the terminal,
	// then keeps drawing for a while so the parent can measure the new frames.
	ptytest.RegisterScenario("spinner-resize", func() (any, error) {
		winch := make(chan os.Signal, 1)
		signal.Notify(winch, syscall.SIGWINCH)
		defer signal.Stop(winch)

		progress := spinner.New(overlongSpinnerText,
			spinner.WithTheme(plainTheme()),
			spinner.WithWriter(os.Stdout),
			spinner.WithInterval(5*time.Millisecond),
		).Start()

		select {
		case <-winch:
		case <-time.After(2 * time.Second):
			return nil, errors.New("the parent never resized the terminal")
		}

		time.Sleep(200 * time.Millisecond)
		progress.Succeed("done")
		return "done", nil
	})
}

// TestSpinnerTracksTheTerminalWidthAfterAResize protects P1-1 for a window that
// shrinks while the spinner runs: every redraw after the resize must fit the
// new width.
func TestSpinnerTracksTheTerminalWidthAfterAResize(t *testing.T) {
	session := ptytest.Start(t, ptytest.Options{
		Scenario: "spinner-resize",
		Cols:     wideColumns + 20,
		Rows:     24,
		Timeout:  3 * time.Second,
		Env:      []string{"TERM=xterm-256color"},
	})

	output := session.Output()
	if _, ok := output.WaitForFunc(func(data string) bool {
		return strings.Count(data, redrawPrefix) >= 2
	}, ptyWait); !ok {
		t.Fatalf("the spinner did not animate before the resize; output: %q", output.String())
	}

	session.Resize(narrowColumns, 24)
	split := len(output.String())

	result := session.Wait()
	if result.Err != "" {
		t.Fatalf("resize scenario: %s", result.Err)
	}
	if got := result.Extra["cols"]; got != "20" {
		t.Fatalf("the child terminal reports %q columns after the resize, want 20", got)
	}

	captured := output.String()
	tail := captured
	if split < len(captured) {
		tail = captured[split:]
	}
	// Drop the frames that may have been in flight while the terminal changed:
	// only complete redraws issued after the resize are measured.
	for skipped := 0; skipped < 2; skipped++ {
		index := strings.Index(tail, redrawPrefix)
		if index < 0 {
			t.Fatalf("no redraw after the resize; output: %q", captured)
		}
		tail = tail[index+len(redrawPrefix):]
	}

	// RED: Relatório P1-1 (C4 corrige). C4 reads the current width before every
	// draw, so the frames that follow a resize already obey it.
	if widest := widestRedraw(tail); widest > narrowColumns {
		t.Errorf("RED: Relatório P1-1 (C4 corrige): after resizing to %d columns the spinner still drew %d cells",
			narrowColumns, widest)
	}
	if ghosts := wrappedLines(tail, narrowColumns); ghosts > 0 {
		t.Errorf("RED: Relatório P1-1 (C4 corrige): %d wrapped lines were left behind after the resize: %q",
			ghosts, tail)
	}
}
