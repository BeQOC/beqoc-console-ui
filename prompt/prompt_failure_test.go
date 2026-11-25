//go:build !windows

package prompt_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"charm.land/huh/v2"
	"github.com/creack/pty"

	"github.com/BeQOC/beqoc-console-ui/internal/ptytest"
	"github.com/BeQOC/beqoc-console-ui/prompt"
	"github.com/BeQOC/beqoc-console-ui/theme"
)

// This file is the red half of the prompt suite: every test here encodes a
// contract from the publication report that the current code breaks. The
// scenarios run in a child process so a hang, a panic or an infinite loop ends
// in the harness watchdog instead of the test binary.
//
// Every red test carries the finding it protects. None of them may be removed
// or relaxed to make the suite green: C3 makes them pass by fixing prompt.

// outcome is what a failure scenario reports: the value the prompt returned,
// the error text, whether a panic happened and which sentinel the error
// satisfied. The parent decides what the contract requires.
type outcome struct {
	Value          any    `json:"value"`
	Error          string `json:"error"`
	Panic          string `json:"panic"`
	Canceled       bool   `json:"canceled"`
	Deadline       bool   `json:"deadline"`
	UserAborted    bool   `json:"userAborted"`
	ErrCanceled    bool   `json:"errCanceled"`
	EOF            bool   `json:"eof"`
	NoChoices      bool   `json:"noChoices"`
	InvalidDefault bool   `json:"invalidDefault"`
}

// guarded runs a prompt call and encodes both the returned error and any panic,
// so the parent can tell a clean error from a crash.
func guarded(run func() (any, error)) (encoded any, err error) {
	var reported outcome
	defer func() {
		if recovered := recover(); recovered != nil {
			reported.Panic = fmt.Sprint(recovered)
		}
		encoded = reported
	}()

	value, runErr := run()
	reported.Value = value
	if runErr != nil {
		reported.Error = runErr.Error()
		reported.Canceled = errors.Is(runErr, context.Canceled)
		reported.Deadline = errors.Is(runErr, context.DeadlineExceeded)
		reported.UserAborted = errors.Is(runErr, huh.ErrUserAborted)
		reported.ErrCanceled = errors.Is(runErr, prompt.ErrCanceled)
		reported.EOF = errors.Is(runErr, prompt.ErrEOF)
		reported.NoChoices = errors.Is(runErr, prompt.ErrNoChoices)
		reported.InvalidDefault = errors.Is(runErr, prompt.ErrInvalidDefault)
	}
	// The deferred assignment is what actually returns the outcome, on both the
	// normal and the recovered-panic paths.
	return nil, nil
}

// scenario adapts a prompt call to the harness. The harness mode decides
// whether the child gets a terminal or pipes; the scenario only names the call.
func scenario(run func() (any, error)) func() (any, error) {
	return func() (any, error) {
		return guarded(run)
	}
}

// signalScenario also prints the child pid, so the parent can send a signal
// once the prompt is on screen, exactly like a shell or a supervisor would.
func signalScenario(run func() (any, error)) func() (any, error) {
	return func() (any, error) {
		fmt.Fprintf(os.Stdout, "pid=%d\n", os.Getpid())
		return guarded(run)
	}
}

func init() {
	// SIGTERM arrives from the parent after the form is rendered.
	ptytest.RegisterScenario("fail-pty-confirm-sigterm", signalScenario(func() (any, error) {
		return ptyPrompter().Confirm(context.Background(), prompt.ConfirmOptions{
			Title:   "Continue?",
			Default: true,
		})
	}))
	ptytest.RegisterScenario("fail-pty-input-sigterm", signalScenario(func() (any, error) {
		return ptyPrompter().Input(context.Background(), prompt.InputOptions{
			Title:   "Project name",
			Default: "def",
		})
	}))
	ptytest.RegisterScenario("fail-pty-select-sigterm", signalScenario(func() (any, error) {
		return prompt.Select(context.Background(), ptyPrompter(), prompt.SelectOptions[string]{
			Title:   "Target",
			Choices: targetChoices(),
		})
	}))

	// Distinctive labels: huh's defaults are "Yes"/"No", so only labels that
	// differ from them prove the options are honoured.
	ptytest.RegisterScenario("fail-pty-confirm-labels", scenario(func() (any, error) {
		return ptyPrompter().Confirm(context.Background(), prompt.ConfirmOptions{
			Title:       "Continue?",
			Description: "The next step writes files.",
			Affirmative: "Sure",
			Negative:    "Nope",
		})
	}))

	// Ctrl-C and Ctrl-D are keys typed on a raw terminal.
	for _, spec := range []struct {
		name string
		run  func() (any, error)
	}{
		{name: "confirm", run: func() (any, error) {
			return ptyPrompter().Confirm(context.Background(), prompt.ConfirmOptions{Title: "Continue?"})
		}},
		{name: "input", run: func() (any, error) {
			return ptyPrompter().Input(context.Background(), prompt.InputOptions{Title: "Project name"})
		}},
		{name: "select", run: func() (any, error) {
			return prompt.Select(context.Background(), ptyPrompter(), prompt.SelectOptions[string]{
				Title:   "Target",
				Choices: targetChoices(),
			})
		}},
	} {
		ptytest.RegisterScenario("fail-pty-"+spec.name+"-ctrlc", scenario(spec.run))
		ptytest.RegisterScenario("fail-pty-"+spec.name+"-ctrld", scenario(spec.run))
	}

	ptytest.RegisterScenario("fail-pty-confirm-ctx-canceled", scenario(func() (any, error) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ptyPrompter().Confirm(ctx, prompt.ConfirmOptions{Title: "Continue?"})
	}))
	ptytest.RegisterScenario("fail-pty-confirm-ctx-deadline", scenario(func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		return ptyPrompter().Confirm(ctx, prompt.ConfirmOptions{Title: "Continue?"})
	}))

	// The child owns this terminal so it can take the master away while the
	// prompt is live, which is what a terminal emulator closing the window does.
	ptytest.RegisterScenario("fail-pty-master-closed", scenario(func() (any, error) {
		master, slave, openErr := pty.Open()
		if openErr != nil {
			return nil, openErr
		}
		defer slave.Close()
		// A real terminal has a size; without one the form never draws.
		if sizeErr := pty.Setsize(master, &pty.Winsize{Cols: 80, Rows: 24}); sizeErr != nil {
			return nil, sizeErr
		}
		prompter := prompt.New(prompt.Options{Theme: ptyTheme(), Input: slave, Output: slave})
		fmt.Fprintln(os.Stdout, "master-closed: rendering on a private terminal")
		go closeTerminalWhenRendered(master, "Continue?")
		return prompter.Confirm(context.Background(), prompt.ConfirmOptions{Title: "Continue?", Default: true})
	}))

	// Accessible mode ignores the context today; the input reaches EOF.
	ptytest.RegisterScenario("fail-pipe-confirm-ctx-canceled", scenario(func() (any, error) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return pipePrompter("", true).Confirm(ctx, prompt.ConfirmOptions{Title: "Continue?"})
	}))

	// Accessible mode, EOF after an answer that must not be accepted.
	ptytest.RegisterScenario("fail-pipe-confirm-maybe-eof", scenario(func() (any, error) {
		return pipePrompter("maybe\n", true).Confirm(context.Background(), prompt.ConfirmOptions{
			Title:   "Continue?",
			Default: true,
		})
	}))
	ptytest.RegisterScenario("fail-pipe-input-validate-eof", scenario(func() (any, error) {
		return pipePrompter("\n", true).Input(context.Background(), prompt.InputOptions{
			Title: "Project name",
			Validate: func(value string) error {
				if value == "" {
					return errors.New("a name is required")
				}
				return nil
			},
		})
	}))

	// Accessible select with answers outside the choices, and with no choices.
	for _, spec := range []struct {
		name    string
		choices []prompt.Choice[string]
		script  string
	}{
		{name: "99-eof", choices: targetChoices(), script: "99\n"},
		{name: "abc-eof", choices: targetChoices(), script: "abc\n"},
		{name: "no-choices", choices: nil, script: "\n"},
	} {
		ptytest.RegisterScenario("fail-pipe-select-"+spec.name, scenario(func() (any, error) {
			return prompt.Select(context.Background(), pipePrompter(spec.script, true), prompt.SelectOptions[string]{
				Title:   "Target",
				Choices: spec.choices,
			})
		}))
	}
	ptytest.RegisterScenario("fail-pipe-select-eof", scenario(func() (any, error) {
		return prompt.Select(context.Background(), pipePrompter("", true), prompt.SelectOptions[string]{
			Title:   "Target",
			Choices: targetChoices(),
		})
	}))
	ptytest.RegisterScenario("fail-pipe-select-bad-default", scenario(func() (any, error) {
		outside := "nope"
		return prompt.Select(context.Background(), pipePrompter("", true), prompt.SelectOptions[string]{
			Title:   "Target",
			Choices: targetChoices(),
			Default: &outside,
		})
	}))

	// Interactive mode on pipes: a non-terminal must fall back to line mode.
	ptytest.RegisterScenario("fail-pipe-plain-input", scenario(func() (any, error) {
		return pipePrompter("hi\n", false).Input(context.Background(), prompt.InputOptions{
			Title: "Project name",
		})
	}))
	// Confirm and Select spin forever on EOF, so their output is capped: the
	// contract is about terminating, not about how much garbage they emit.
	ptytest.RegisterScenario("fail-pipe-plain-confirm", scenario(func() (any, error) {
		return pipePrompterWith("", false, newCappedWriter(os.Stdout, 512)).Confirm(
			context.Background(), prompt.ConfirmOptions{Title: "Continue?"})
	}))
	ptytest.RegisterScenario("fail-pipe-plain-select", scenario(func() (any, error) {
		return prompt.Select(context.Background(), pipePrompterWith("", false, newCappedWriter(os.Stdout, 512)),
			prompt.SelectOptions[string]{Title: "Target", Choices: targetChoices()})
	}))
}

// ptyTheme keeps prompt assertions deterministic regardless of the environment.
func ptyTheme() *theme.Theme {
	return theme.New(theme.WithProfile(theme.ProfileNone))
}

// pipePrompter builds an accessible or interactive prompter that reads script
// from a real pipe and then sees EOF.
func pipePrompter(script string, accessible bool) *prompt.Prompter {
	return pipePrompterWith(script, accessible, os.Stdout)
}

func pipePrompterWith(script string, accessible bool, output io.Writer) *prompt.Prompter {
	return prompt.New(prompt.Options{
		Theme:      ptyTheme(),
		Input:      scriptedPipe(script),
		Output:     output,
		Accessible: accessible,
	})
}

// scriptedPipe writes the whole script, closes the write end and returns the
// read end, so the prompt sees the script followed by EOF.
func scriptedPipe(script string) *os.File {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	if _, err := io.WriteString(writer, script); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return reader
}

// cappedWriter keeps the first bytes a looping program writes and discards the
// rest, so a prompt stuck in an infinite loop cannot exhaust memory.
type cappedWriter struct {
	out  io.Writer
	left int
}

func newCappedWriter(out io.Writer, limit int) *cappedWriter {
	return &cappedWriter{out: out, left: limit}
}

func (w *cappedWriter) Write(data []byte) (int, error) {
	if w.left > 0 {
		kept := data
		if len(kept) > w.left {
			kept = kept[:w.left]
		}
		if _, err := w.out.Write(kept); err != nil {
			return 0, err
		}
		w.left -= len(kept)
	}
	return len(data), nil
}

// closeTerminalWhenRendered takes the master away as soon as the prompt has
// drawn its title on the terminal.
func closeTerminalWhenRendered(master *os.File, marker string) {
	var seen []byte
	buffer := make([]byte, 1024)
	for {
		n, err := master.Read(buffer)
		seen = append(seen, buffer[:n]...)
		if len(seen) > 64*1024 {
			_ = master.Close()
			return
		}
		if n > 0 && strings.Contains(string(seen), marker) {
			fmt.Fprintln(os.Stdout, "master-closed: the terminal is gone")
			_ = master.Close()
			return
		}
		if err != nil {
			fmt.Fprintln(os.Stdout, "master-closed: the terminal failed before rendering")
			_ = master.Close()
			return
		}
	}
}

var (
	pidPattern = regexp.MustCompile(`pid=(\d+)`)
	// ansiPattern matches the escape sequences a terminal consumes, so an
	// assertion can look at the text a person would actually read.
	ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
)

// visibleText removes terminal escape sequences from captured output.
func visibleText(output string) string {
	return ansiPattern.ReplaceAllString(output, "")
}

func childPID(t *testing.T, output string) int {
	t.Helper()
	match := pidPattern.FindStringSubmatch(output)
	if match == nil {
		t.Fatalf("the child did not print its pid; output: %q", output)
	}
	pid, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("parse child pid %q: %v", match[1], err)
	}
	return pid
}

func startFailurePTY(t *testing.T, scenario string) *ptytest.Session {
	t.Helper()
	return ptytest.Start(t, ptytest.Options{
		Scenario: scenario,
		Timeout:  3 * time.Second,
		Env:      []string{"TERM=xterm-256color"},
	})
}

func startFailurePipe(t *testing.T, scenario string) *ptytest.Session {
	t.Helper()
	return ptytest.Start(t, ptytest.Options{
		Scenario: scenario,
		Mode:     ptytest.Pipe,
		Timeout:  3 * time.Second,
		Env:      []string{"TERM=xterm-256color"},
	})
}

func decodeOutcome(t *testing.T, result ptytest.Result) outcome {
	t.Helper()
	if result.Err != "" {
		t.Fatalf("the scenario failed before the prompt completed: %s", result.Err)
	}
	raw, err := json.Marshal(result.Value)
	if err != nil {
		t.Fatalf("encode scenario value: %v", err)
	}
	var decoded outcome
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode scenario value %s: %v", raw, err)
	}
	return decoded
}

func expectNoPanic(t *testing.T, reported outcome) {
	t.Helper()
	if reported.Panic != "" {
		t.Errorf("the prompt panicked instead of returning: %s", reported.Panic)
	}
}

// TestPTYSIGTERMNeverBecomesAnAnswer protects the first P0: a signal delivered
// while a form is open must never complete the form with its current value.
func TestPTYSIGTERMNeverBecomesAnAnswer(t *testing.T) {
	cases := []struct {
		name     string
		scenario string
		ready    string
	}{
		{name: "confirm with a default", scenario: "fail-pty-confirm-sigterm", ready: "Continue?"},
		{name: "input with a default", scenario: "fail-pty-input-sigterm", ready: "Project name"},
		{name: "select", scenario: "fail-pty-select-sigterm", ready: "Target"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			session := startFailurePTY(t, testCase.scenario)
			output := session.WaitFor(testCase.ready)

			pid := childPID(t, output)
			if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
				t.Fatalf("signal the child: %v", err)
			}

			reported := decodeOutcome(t, session.Wait())
			expectNoPanic(t, reported)

			// RED: Relatório P0-1 (C3 corrige). Today SIGTERM returns the
			// current or default value as a silent success.
			if reported.Error == "" {
				t.Errorf("SIGTERM returned (%v, nil); a signal must never become an answer", reported.Value)
			}
			// C3 fortalece para errors.Is(err, prompt.ErrCanceled).
		})
	}
}

// TestPTYCtrlCIsNotAHuhError protects P0-6a: Ctrl-C must cancel, not leak the
// form library's error type into the caller.
func TestPTYCtrlCIsNotAHuhError(t *testing.T) {
	cases := []struct {
		name     string
		scenario string
		ready    string
	}{
		{name: "confirm", scenario: "fail-pty-confirm-ctrlc", ready: "Continue?"},
		{name: "input", scenario: "fail-pty-input-ctrlc", ready: "Project name"},
		{name: "select", scenario: "fail-pty-select-ctrlc", ready: "Target"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			session := startFailurePTY(t, testCase.scenario)
			session.WaitFor(testCase.ready)
			initial, err := session.InitialTermios()
			if err != nil {
				t.Fatalf("initial terminal state: %v", err)
			}

			session.Send("\x03")
			reported := decodeOutcome(t, session.Wait())
			expectNoPanic(t, reported)

			// RED: Relatório P0-6a (C3 corrige). Today the error is
			// huh.ErrUserAborted, which couples every caller to huh.
			if reported.Error == "" {
				t.Errorf("Ctrl-C returned (%v, nil); want a cancellation error", reported.Value)
			}
			if reported.UserAborted {
				t.Errorf("Ctrl-C leaked huh.ErrUserAborted; C3 normalizes it to prompt.ErrCanceled")
			}
			// C3 fortalece para errors.Is(err, prompt.ErrCanceled).

			// This half is green today: the matrix shows the terminal restored
			// after Ctrl-C, and it must stay that way.
			final, err := session.Termios()
			if err != nil {
				t.Fatalf("terminal state after Ctrl-C: %v", err)
			}
			if final != initial {
				t.Errorf("terminal state after Ctrl-C = %v, want %v", final, initial)
			}
		})
	}
}

// TestPTYCtrlDTerminates protects P0-6d: Ctrl-D must end the prompt with an
// error. Today the form hangs and the harness watchdog kills it, so this test
// fails at the watchdog with the partial output.
func TestPTYCtrlDTerminates(t *testing.T) {
	cases := []struct {
		name     string
		scenario string
		ready    string
	}{
		{name: "confirm", scenario: "fail-pty-confirm-ctrld", ready: "Continue?"},
		{name: "input", scenario: "fail-pty-input-ctrld", ready: "Project name"},
		{name: "select", scenario: "fail-pty-select-ctrld", ready: "Target"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			session := startFailurePTY(t, testCase.scenario)
			session.WaitFor(testCase.ready)
			session.Send("\x04")

			// RED: Relatório P0-6d (C3 corrige). Today Wait fails here at the
			// 3s watchdog because the form never returns.
			reported := decodeOutcome(t, session.Wait())
			expectNoPanic(t, reported)

			if reported.Error == "" {
				t.Errorf("Ctrl-D returned (%v, nil); want an error, never an answer", reported.Value)
			}
			// C3 fortalece para errors.Is(err, prompt.ErrEOF) ou ErrCanceled.
		})
	}
}

// TestPTYConfirmHonorsAPreCancelledContext protects P0-6b: the caller's
// cancellation must survive as context.Canceled, not huh's "timeout".
func TestPTYConfirmHonorsAPreCancelledContext(t *testing.T) {
	session := startFailurePTY(t, "fail-pty-confirm-ctx-canceled")
	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)

	// RED: Relatório P0-6b (C3 corrige). Today huh reports its own ErrTimeout.
	if !reported.Canceled {
		t.Errorf("pre-cancelled context returned (%v, %q); want errors.Is(err, context.Canceled)",
			reported.Value, reported.Error)
	}
}

// TestPTYConfirmHonorsAnExpiredDeadline protects P0-6b for deadlines, which are
// cancellations too.
func TestPTYConfirmHonorsAnExpiredDeadline(t *testing.T) {
	session := startFailurePTY(t, "fail-pty-confirm-ctx-deadline")
	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)

	// RED: Relatório P0-6b (C3 corrige).
	if !reported.Deadline {
		t.Errorf("expired deadline returned (%v, %q); want errors.Is(err, context.DeadlineExceeded)",
			reported.Value, reported.Error)
	}
}

// TestPipeConfirmHonorsAPreCancelledContext protects P0-6c: accessible mode
// ignores the context completely today.
func TestPipeConfirmHonorsAPreCancelledContext(t *testing.T) {
	session := startFailurePipe(t, "fail-pipe-confirm-ctx-canceled")
	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)

	// RED: Relatório P0-6c (C3 corrige).
	if !reported.Canceled {
		t.Errorf("accessible confirm with a cancelled context returned (%v, %q); want context.Canceled",
			reported.Value, reported.Error)
	}
	if !reported.ErrCanceled {
		t.Errorf("error %q does not match prompt.ErrCanceled", reported.Error)
	}
}

// TestPTYReturnsWhenTheTerminalDisappears covers the "master closed" row of
// matrix 1.1: a program whose terminal vanishes must not hang forever.
func TestPTYReturnsWhenTheTerminalDisappears(t *testing.T) {
	session := startFailurePTY(t, "fail-pty-master-closed")

	// RED: matriz 1.1 "Master PTY fechado" (C3 corrige). Today Wait fails here
	// at the 3s watchdog because the prompt waits on a dead terminal.
	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)

	if reported.Error == "" {
		t.Errorf("a prompt whose terminal disappeared returned (%v, nil); want an error", reported.Value)
	}
}

// TestPipeConfirmRejectsAnInvalidAnswerAtEOF protects P0-3: EOF must never turn
// an invalid answer into a silent "no".
func TestPipeConfirmRejectsAnInvalidAnswerAtEOF(t *testing.T) {
	session := startFailurePipe(t, "fail-pipe-confirm-maybe-eof")
	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)

	// RED: Relatório P0-3 (C3 corrige). Today it returns (false, nil), ignoring
	// both the default and the invalid answer.
	if reported.Error == "" {
		t.Errorf("invalid answer followed by EOF returned (%v, nil); want an error", reported.Value)
	}
	// C3 fortalece para errors.Is(err, prompt.ErrEOF).
	if !reported.EOF {
		t.Errorf("error %q does not match prompt.ErrEOF", reported.Error)
	}
}

// TestPipeInputValidatesBeforeEOF protects P0-3: EOF after a rejected answer
// must not bypass Validate.
func TestPipeInputValidatesBeforeEOF(t *testing.T) {
	session := startFailurePipe(t, "fail-pipe-input-validate-eof")
	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)

	// RED: Relatório P0-3 (C3 corrige). Today it returns ("", nil), accepting
	// the value Validate rejected.
	if reported.Error == "" {
		t.Errorf("EOF after a rejected answer returned (%v, nil); want an error", reported.Value)
	}
	// C3 fortalece para errors.Is(err, prompt.ErrEOF).
	if !reported.EOF {
		t.Errorf("error %q does not match prompt.ErrEOF", reported.Error)
	}
}

// TestPipeSelectSurvivesBadAnswers protects P0-2 and P0-3: answers outside the
// choices and an empty choice list must return an error, never panic.
func TestPipeSelectSurvivesBadAnswers(t *testing.T) {
	cases := []struct {
		name     string
		scenario string
		noChoice bool
	}{
		{name: "index out of range", scenario: "fail-pipe-select-99-eof"},
		{name: "not a number", scenario: "fail-pipe-select-abc-eof"},
		{name: "no choices", scenario: "fail-pipe-select-no-choices", noChoice: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			session := startFailurePipe(t, testCase.scenario)
			reported := decodeOutcome(t, session.Wait())

			// RED: Relatório P0-2/P0-3 (C3 corrige). Today the child panics with
			// "index out of range".
			if reported.Panic != "" {
				t.Errorf("select panicked instead of returning an error: %s", reported.Panic)
			}
			if reported.Error == "" {
				t.Errorf("select returned (%v, nil); want an error", reported.Value)
			}
			// C3 fortalece para errors.Is(err, prompt.ErrNoChoices) na lista vazia.
			if testCase.noChoice {
				if !reported.NoChoices {
					t.Errorf("error %q does not match prompt.ErrNoChoices", reported.Error)
				}
			} else if !reported.EOF {
				t.Errorf("error %q does not match prompt.ErrEOF", reported.Error)
			}
		})
	}
}

// TestPipeSelectRejectsEOFWithoutADefault protects P0-3: EOF must not pick the
// first choice as if the user had chosen it.
func TestPipeSelectRejectsEOFWithoutADefault(t *testing.T) {
	session := startFailurePipe(t, "fail-pipe-select-eof")
	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)

	// RED: Relatório P0-3 (C3 corrige). Today it returns ("alpha", nil).
	if reported.Error == "" {
		t.Errorf("EOF returned the first choice (%v, nil); want an error", reported.Value)
	}
	if !reported.EOF {
		t.Errorf("error %q does not match prompt.ErrEOF", reported.Error)
	}
}

// TestPipeSelectRejectsADefaultOutsideTheChoices protects P0-2: a default that
// is not an option must be an error, not a silent first choice.
func TestPipeSelectRejectsADefaultOutsideTheChoices(t *testing.T) {
	session := startFailurePipe(t, "fail-pipe-select-bad-default")
	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)

	// RED: Relatório P0-2 (C3 corrige). Today it returns ("alpha", nil).
	if reported.Error == "" {
		t.Errorf("a default outside the choices returned (%v, nil); want an error", reported.Value)
	}
	// C3 fortalece para errors.Is(err, prompt.ErrInvalidDefault).
	if !reported.InvalidDefault {
		t.Errorf("error %q does not match prompt.ErrInvalidDefault", reported.Error)
	}
}

// TestPipeInteractiveInputFallsBackToLineMode protects P0-4: a prompt whose
// streams are not terminals must read lines instead of starting a form.
func TestPipeInteractiveInputFallsBackToLineMode(t *testing.T) {
	session := startFailurePipe(t, "fail-pipe-plain-input")
	reported := decodeOutcome(t, session.Wait())

	// RED: Relatório P0-4 (C3 corrige). Today the child panics twice.
	if reported.Panic != "" {
		t.Errorf("the prompt panicked: %s", reported.Panic)
	}
	if reported.Error != "" {
		t.Errorf("input returned %q, want nil", reported.Error)
	}
	if value, ok := reported.Value.(string); !ok || value != "hi" {
		t.Errorf("input = %#v, want hi", reported.Value)
	}
	if output := session.Output().String(); strings.Contains(output, "Caught panic") {
		t.Errorf("a panic trace reached the output; a non-terminal must use line mode: %q", output)
	}
}

// TestPipeInteractivePromptsDoNotLoopOnEOF protects P0-4: Confirm and Select on
// a pipe must terminate. Today they spin emitting "\x1b[J" until the watchdog.
func TestPipeInteractivePromptsDoNotLoopOnEOF(t *testing.T) {
	cases := []struct {
		name     string
		scenario string
	}{
		{name: "confirm", scenario: "fail-pipe-plain-confirm"},
		{name: "select", scenario: "fail-pipe-plain-select"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			session := startFailurePipe(t, testCase.scenario)

			// RED: Relatório P0-4 (C3 corrige). Today Wait fails here at the 3s
			// watchdog, whose message carries the captured "\x1b[J" flood.
			reported := decodeOutcome(t, session.Wait())
			expectNoPanic(t, reported)

			if reported.Error == "" {
				t.Errorf("EOF on a pipe returned (%v, nil); want an error", reported.Value)
			}
			if !reported.EOF {
				t.Errorf("error %q does not match prompt.ErrEOF", reported.Error)
			}
		})
	}
}

// TestPTYConfirmRendersDescriptionAndAnswerLabels is green today and keeps the
// Description, Affirmative and Negative options honest where huh renders them:
// the accessible renderer prints only the title, so this is the only place a
// caller can see them. Mutations that drop them (section 6.1) fail here.
func TestPTYConfirmRendersDescriptionAndAnswerLabels(t *testing.T) {
	session := startFailurePTY(t, "fail-pty-confirm-labels")

	session.WaitFor("Continue?")
	session.SendLine("y")
	reported := decodeOutcome(t, session.Wait())
	if value, ok := reported.Value.(bool); !ok || !value {
		t.Fatalf("confirm = %#v, want true", reported.Value)
	}

	output := visibleText(session.Output().String())
	for _, want := range []string{"The next step writes files.", "Sure", "Nope"} {
		if !strings.Contains(output, want) {
			t.Errorf("prompt output does not show %q: %q", want, output)
		}
	}
}

// TestPTYInputRendersThePlaceholder is green today and keeps Placeholder
// honest where huh renders it. The accessible renderer prints only the title,
// so an interactive prompt is the only observable place. Mutations that drop
// Placeholder (section 6.1) fail here.
func TestPTYInputRendersThePlaceholder(t *testing.T) {
	session := startFailurePTY(t, "input")

	output := visibleText(session.WaitFor("Project name"))
	if !strings.Contains(output, "beqoc") {
		t.Errorf("prompt output does not show the placeholder %q: %q", "beqoc", output)
	}

	session.SendLine("beqoc")
	result := session.Wait()
	if got := resultString(t, result); got != "beqoc" {
		t.Fatalf("input = %q, want beqoc", got)
	}
}
