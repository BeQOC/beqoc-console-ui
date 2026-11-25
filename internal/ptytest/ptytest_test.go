//go:build !windows

package ptytest

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestPTYHelperProcess(t *testing.T) {
	HelperProcess(t)
}

func init() {
	RegisterScenario("echo-line", func() (any, error) {
		fmt.Fprintln(os.Stdout, "ready")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stdout, "got: %s", line)
		return strings.TrimSpace(line), nil
	})
	RegisterScenario("wait-winch", func() (any, error) {
		winch := make(chan os.Signal, 1)
		signal.Notify(winch, syscall.SIGWINCH)
		defer signal.Stop(winch)
		fmt.Fprintln(os.Stdout, "ready")
		select {
		case <-winch:
			return "winched", nil
		case <-time.After(5 * time.Second):
			return nil, errors.New("no SIGWINCH")
		}
	})
	RegisterScenario("hang", func() (any, error) {
		time.Sleep(time.Hour)
		return nil, nil
	})
}

func TestPTYEchoesALineAndReportsTheResult(t *testing.T) {
	session := Start(t, Options{Scenario: "echo-line"})

	session.WaitFor("ready")
	session.SendLine("hello")
	result := session.Wait()

	if result.Err != "" {
		t.Fatalf("scenario error = %q", result.Err)
	}
	if result.Value != "hello" {
		t.Fatalf("value = %#v, want hello", result.Value)
	}
	if output := session.Output().String(); !strings.Contains(output, "got: hello") {
		t.Fatalf("output = %q, want the echoed line", output)
	}
	if got := result.Extra["cols"]; got != "80" {
		t.Fatalf("reported cols = %q, want 80", got)
	}
	// The child reports its own terminal state; macOS revokes the slave once the
	// session leader exits, so this is what the parent reads after a run there.
	for _, flag := range []string{"icanon", "echo", "isig"} {
		if got := result.Extra[flag]; got != "true" {
			t.Fatalf("reported %s = %q, want true (the scenario leaves a cooked terminal)", flag, got)
		}
	}

	initial, err := session.InitialTermios()
	if err != nil {
		t.Fatalf("initial terminal state: %v", err)
	}
	if !initial.Canonical || !initial.Echo || !initial.Signals {
		t.Fatalf("initial terminal state = %v, want a cooked terminal", initial)
	}
	final, err := session.Termios()
	if err != nil {
		t.Fatalf("terminal state: %v", err)
	}
	if final != initial {
		t.Fatalf("terminal state = %v, want %v", final, initial)
	}
}

func TestPTYResizeReachesTheChild(t *testing.T) {
	session := Start(t, Options{Scenario: "wait-winch", Cols: 80, Rows: 24})

	session.WaitFor("ready")
	session.Resize(100, 30)
	result := session.Wait()

	if result.Err != "" {
		t.Fatalf("scenario error = %q", result.Err)
	}
	if result.Value != "winched" {
		t.Fatalf("value = %#v, want winched", result.Value)
	}
	if got := result.Extra["cols"]; got != "100" {
		t.Fatalf("reported cols = %q, want 100", got)
	}
}

func TestPipeEchoesALineAndReportsTheResult(t *testing.T) {
	session := Start(t, Options{Scenario: "echo-line", Mode: Pipe})

	session.WaitFor("ready")
	session.SendLine("hello")
	result := session.Wait()

	if result.Err != "" {
		t.Fatalf("scenario error = %q", result.Err)
	}
	if result.Value != "hello" {
		t.Fatalf("value = %#v, want hello", result.Value)
	}
	if output := session.Output().String(); !strings.Contains(output, "got: hello") {
		t.Fatalf("output = %q, want the echoed line", output)
	}
	if _, ok := result.Extra["cols"]; ok {
		t.Fatalf("a pipe reported a terminal size: %#v", result.Extra)
	}
}

func TestTerminalCapturesWhatTheSlaveWrites(t *testing.T) {
	terminal := OpenTerminal(t, 33, 12)

	size, err := pty.GetsizeFull(terminal.Master())
	if err != nil {
		t.Fatalf("terminal size: %v", err)
	}
	if size.Cols != 33 || size.Rows != 12 {
		t.Fatalf("terminal size = %dx%d, want 33x12", size.Cols, size.Rows)
	}

	if _, err := io.WriteString(terminal.Slave(), "hello\nthere\n"); err != nil {
		t.Fatalf("write to the slave: %v", err)
	}
	output, ok := terminal.Output().WaitFor("there", 5*time.Second)
	if !ok {
		t.Fatalf("the terminal did not deliver the written line; output: %q", output)
	}
	// The terminal driver turns a newline into a carriage return plus newline,
	// which is why assertions on captured output must tolerate the CR.
	if !strings.Contains(output, "hello\r\nthere") {
		t.Fatalf("output = %q, want cooked line endings", output)
	}
}

func TestWatchdogKillsAChildThatNeverFinishes(t *testing.T) {
	session := Start(t, Options{Scenario: "hang", Timeout: 200 * time.Millisecond})

	done, _ := session.reap(200 * time.Millisecond)
	if done {
		t.Fatal("the hanging child finished before the watchdog")
	}

	session.kill()
	done, _ = session.reap(2 * time.Second)
	if !done {
		t.Fatal("the child did not exit after the watchdog killed it")
	}
}

func TestTermiosFallsBackToTheChildReport(t *testing.T) {
	// A pipe is not a terminal, so the parent-side read fails exactly like the
	// revoked slave on macOS; the state the child reported must be used instead.
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()

	session := &Session{
		slave:       reader,
		resultReady: true,
		result: Result{Extra: map[string]string{
			"icanon": "false",
			"echo":   "false",
			"isig":   "false",
		}},
	}

	state, err := session.Termios()
	if err != nil {
		t.Fatalf("terminal state: %v", err)
	}
	if state.Canonical || state.Echo || state.Signals {
		t.Fatalf("terminal state = %v, want the child-reported raw state", state)
	}
}
