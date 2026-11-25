//go:build !windows

// Package ptytest drives real pseudo-terminals and plain pipes from tests.
//
// A scenario runs in a child process: the parent test binary re-executes itself
// with [HelperTestName] selected and one environment variable naming the
// scenario. In PTY mode the child's standard streams are the slave side of a
// real pseudo-terminal while the parent keeps the master side, so the test can
// type keys, resize the window and read what the program rendered. In Pipe mode
// the child reads and writes plain pipes, which is how a script or a CI job
// feeds a program.
//
// The child reports one JSON value through an extra file descriptor, so the
// parent reads a typed result instead of parsing terminal output. Every wait has
// a deadline and a watchdog kills a child that never finishes, so a broken
// prompt fails the test with partial output instead of hanging the suite.
//
// A package that uses this harness must expose the child entry point:
//
//	func TestPTYHelperProcess(t *testing.T) { ptytest.HelperProcess(t) }
//
// and register its scenarios with [RegisterScenario] from an init function in a
// test file.
package ptytest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

const (
	// HelperTestName is the test function a package must expose so this
	// harness can re-execute its test binary in child mode.
	HelperTestName = "TestPTYHelperProcess"

	childEnv    = "PTYTEST_CHILD"
	scenarioEnv = "PTYTEST_SCENARIO"
	// resultFD is the file descriptor the child writes its result to. It is
	// the first entry of exec.Cmd.ExtraFiles, which the runtime numbers from 3.
	resultFD = 3

	defaultTimeout = 5 * time.Second
	defaultCols    = 80
	defaultRows    = 24
)

// Result is what the child reports about the code it ran.
type Result struct {
	// Value is the JSON form of the scenario's value: strings and booleans
	// keep their type, numbers decode as float64.
	Value any `json:"value"`
	// Err is the scenario error message, empty on success.
	Err string `json:"err,omitempty"`
	// Extra carries harness facts. The child fills "cols" and "rows" with the
	// terminal size when its output is a terminal.
	Extra map[string]string `json:"extra,omitempty"`
}

// Handler runs a scenario in the child process. Its input and output are the
// process standard streams, already attached to the terminal or the pipes.
type Handler func() (any, error)

var (
	scenariosMu sync.RWMutex
	scenarios   = make(map[string]Handler)
)

// RegisterScenario makes a child scenario available by name. Register from an
// init function in a test file so the child process sees it too.
func RegisterScenario(name string, handler Handler) {
	if name == "" {
		panic("ptytest: empty scenario name")
	}
	if handler == nil {
		panic("ptytest: nil handler for scenario " + name)
	}
	scenariosMu.Lock()
	defer scenariosMu.Unlock()
	if _, exists := scenarios[name]; exists {
		panic("ptytest: scenario " + name + " registered twice")
	}
	scenarios[name] = handler
}

func lookupScenario(name string) (Handler, bool) {
	scenariosMu.RLock()
	defer scenariosMu.RUnlock()
	handler, ok := scenarios[name]
	return handler, ok
}

// HelperProcess is the child entry point. It does nothing in the parent test
// binary and runs the requested scenario, then exits, in the child.
func HelperProcess(t *testing.T) {
	if os.Getenv(childEnv) != "1" {
		return
	}
	t.Helper()
	// The child exists only to run this scenario, so exiting here is the
	// point: it keeps the test framework from writing its own output to the
	// captured terminal and ends the process as soon as the scenario is done.
	os.Exit(runChild())
}

// runChild executes the scenario named in the environment and writes its
// result. It returns the process exit code.
func runChild() int {
	name := os.Getenv(scenarioEnv)
	handler, ok := lookupScenario(name)
	if !ok {
		if err := writeResult(Result{Err: "ptytest: unknown scenario " + strconv.Quote(name)}); err != nil {
			fmt.Fprintf(os.Stderr, "ptytest: %v\n", err)
		}
		return 2
	}

	value, err := handler()
	result := Result{Value: value}
	if err != nil {
		result.Err = err.Error()
	}
	if size, sizeErr := pty.GetsizeFull(os.Stdout); sizeErr == nil {
		result.Extra = map[string]string{
			"cols": strconv.Itoa(int(size.Cols)),
			"rows": strconv.Itoa(int(size.Rows)),
		}
	}
	if err := writeResult(result); err != nil {
		fmt.Fprintf(os.Stderr, "ptytest: %v\n", err)
		return 2
	}
	return 0
}

func writeResult(result Result) error {
	file := os.NewFile(resultFD, "ptytest-result")
	if file == nil {
		return errors.New("ptytest: result descriptor is not open")
	}
	if err := json.NewEncoder(file).Encode(result); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// Mode selects how the child process is attached.
type Mode int

const (
	// PTY attaches the child to a real pseudo-terminal.
	PTY Mode = iota
	// Pipe attaches the child to plain pipes, so its standard streams are not
	// terminals.
	Pipe
)

// Options configure a child process.
type Options struct {
	// Scenario names a handler registered with [RegisterScenario].
	Scenario string
	// Mode selects the terminal or the pipes. It defaults to [PTY].
	Mode Mode
	// Cols and Rows size the pseudo-terminal. They default to 80x24 and are
	// ignored in Pipe mode.
	Cols int
	Rows int
	// Timeout bounds every wait and the watchdog that kills a child which
	// never finishes. It defaults to 5s.
	Timeout time.Duration
	// Env adds environment variables for the child. Later entries win over
	// the parent environment.
	Env []string
}

// Session is a child process attached to a terminal or to pipes.
type Session struct {
	t    *testing.T
	opts Options

	cmd    *exec.Cmd
	master *os.File // PTY mode: parent side of the terminal.
	slave  *os.File // PTY mode: child side, kept open for terminal state.
	input  io.Writer

	stdin   *os.File // Pipe mode: parent write end.
	stdinR  *os.File // Pipe mode: child read end, closed after start.
	stdoutR *os.File // Pipe mode: parent read end.
	stdoutW *os.File // Pipe mode: child write end, closed after start.

	resultR *os.File
	out     *Output

	initial    Termios
	initialErr error

	waitCh  chan error
	reapMu  sync.Mutex
	reaped  bool
	waitErr error

	closeOnce sync.Once
}

// Start launches the child process for a scenario.
func Start(t *testing.T, opts Options) *Session {
	t.Helper()
	if _, ok := lookupScenario(opts.Scenario); !ok {
		t.Fatalf("ptytest: scenario %q is not registered", opts.Scenario)
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	if opts.Cols <= 0 {
		opts.Cols = defaultCols
	}
	if opts.Rows <= 0 {
		opts.Rows = defaultRows
	}

	resultR, resultW, err := os.Pipe()
	if err != nil {
		t.Fatalf("ptytest: create result pipe: %v", err)
	}

	session := &Session{t: t, opts: opts, resultR: resultR}

	cmd := exec.Command(os.Args[0], "-test.run=^"+HelperTestName+"$", "-test.timeout=0")
	cmd.Env = append(os.Environ(), childEnv+"=1", scenarioEnv+"="+opts.Scenario)
	cmd.Env = append(cmd.Env, opts.Env...)
	cmd.ExtraFiles = []*os.File{resultW}

	var stream *os.File
	switch opts.Mode {
	case PTY:
		master, slave, openErr := pty.Open()
		if openErr != nil {
			_ = resultR.Close()
			_ = resultW.Close()
			t.Fatalf("ptytest: open pseudo-terminal: %v", openErr)
		}
		size := &pty.Winsize{Cols: uint16(opts.Cols), Rows: uint16(opts.Rows)}
		if sizeErr := pty.Setsize(slave, size); sizeErr != nil {
			_ = master.Close()
			_ = slave.Close()
			_ = resultR.Close()
			_ = resultW.Close()
			t.Fatalf("ptytest: set terminal size: %v", sizeErr)
		}
		cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
		// A session leader with the slave as its controlling terminal is what
		// a program started from a shell would see.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
		session.master, session.slave = master, slave
		session.input = master
		session.initial, session.initialErr = readTermios(slave)
		stream = master
	case Pipe:
		stdinR, stdinW, stdinErr := os.Pipe()
		if stdinErr != nil {
			_ = resultR.Close()
			_ = resultW.Close()
			t.Fatalf("ptytest: create stdin pipe: %v", stdinErr)
		}
		stdoutR, stdoutW, stdoutErr := os.Pipe()
		if stdoutErr != nil {
			_ = stdinR.Close()
			_ = stdinW.Close()
			_ = resultR.Close()
			_ = resultW.Close()
			t.Fatalf("ptytest: create stdout pipe: %v", stdoutErr)
		}
		cmd.Stdin, cmd.Stdout, cmd.Stderr = stdinR, stdoutW, stdoutW
		session.stdin, session.stdinR = stdinW, stdinR
		session.stdoutR, session.stdoutW = stdoutR, stdoutW
		session.input = stdinW
		stream = stdoutR
	default:
		_ = resultR.Close()
		_ = resultW.Close()
		t.Fatalf("ptytest: unknown mode %d", opts.Mode)
	}

	if err := cmd.Start(); err != nil {
		_ = resultW.Close()
		session.closeFiles()
		t.Fatalf("ptytest: start child: %v", err)
	}
	session.cmd = cmd
	session.waitCh = make(chan error, 1)
	go func() { session.waitCh <- cmd.Wait() }()

	// The parent must not keep the child's ends open: the result pipe has to
	// reach EOF, and a lingering stdin write end would hide a closed pipe.
	_ = resultW.Close()
	if opts.Mode == Pipe {
		_ = session.stdinR.Close()
		_ = session.stdoutW.Close()
		session.stdinR, session.stdoutW = nil, nil
	}

	session.out = capture(stream)
	t.Cleanup(session.Close)
	return session
}

// Output returns everything the child wrote so far, with wait helpers.
func (s *Session) Output() *Output {
	return s.out
}

// Send writes raw bytes to the child: keystrokes in PTY mode, standard input in
// Pipe mode.
func (s *Session) Send(text string) {
	s.t.Helper()
	if _, err := io.WriteString(s.input, text); err != nil {
		s.t.Fatalf("ptytest: send %q: %v", text, err)
	}
}

// SendLine writes text followed by Enter: a carriage return on a terminal,
// which is what a raw terminal delivers, and a newline on a pipe.
func (s *Session) SendLine(text string) {
	s.t.Helper()
	if s.opts.Mode == Pipe {
		s.Send(text + "\n")
		return
	}
	s.Send(text + "\r")
}

// WaitFor waits until the child's output contains pattern and returns it. It
// fails the test when the pattern does not appear before the timeout.
func (s *Session) WaitFor(pattern string) string {
	s.t.Helper()
	out, ok := s.out.WaitFor(pattern, s.opts.Timeout)
	if !ok {
		s.t.Fatalf("ptytest: %q did not appear within %s; output: %q", pattern, s.opts.Timeout, out)
	}
	return out
}

// Resize changes the pseudo-terminal window size and signals the child, which
// is what a terminal emulator does when its window changes.
func (s *Session) Resize(cols, rows int) {
	s.t.Helper()
	if s.slave == nil {
		s.t.Fatal("ptytest: Resize needs a pseudo-terminal")
	}
	if cols <= 0 || rows <= 0 {
		s.t.Fatalf("ptytest: invalid size %dx%d", cols, rows)
	}
	size := &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}
	if err := pty.Setsize(s.slave, size); err != nil {
		s.t.Fatalf("ptytest: resize terminal: %v", err)
	}
	if err := s.cmd.Process.Signal(syscall.SIGWINCH); err != nil && !errors.Is(err, os.ErrProcessDone) {
		s.t.Fatalf("ptytest: signal SIGWINCH: %v", err)
	}
}

// Wait waits for the child to finish and returns its report. It fails the test
// when the watchdog expires or when the child exits without a report.
func (s *Session) Wait() Result {
	s.t.Helper()
	if ok, _ := s.reap(s.opts.Timeout); !ok {
		partial := s.out.String()
		s.kill()
		s.t.Fatalf("ptytest: scenario %q did not finish within %s; output: %q", s.opts.Scenario, s.opts.Timeout, partial)
	}

	raw, err := io.ReadAll(s.resultR)
	if err != nil {
		s.t.Fatalf("ptytest: read result: %v", err)
	}
	_ = s.resultR.Close()
	s.resultR = nil

	if len(strings.TrimSpace(string(raw))) == 0 {
		s.t.Fatalf("ptytest: scenario %q exited with %v and no result; output: %q", s.opts.Scenario, s.waitErr, s.out.String())
	}
	var result Result
	if err := json.Unmarshal(raw, &result); err != nil {
		s.t.Fatalf("ptytest: decode result %q: %v", raw, err)
	}
	return result
}

// WaitForExit waits for the child to end and returns its final state. Unlike
// [Session.Wait] it does not require a result, so a test can assert how the
// child terminated: an exit code, or a signal.
func (s *Session) WaitForExit() *os.ProcessState {
	s.t.Helper()
	if ok, _ := s.reap(s.opts.Timeout); !ok {
		partial := s.out.String()
		s.kill()
		s.t.Fatalf("ptytest: scenario %q did not exit within %s; output: %q", s.opts.Scenario, s.opts.Timeout, partial)
	}
	return s.cmd.ProcessState
}

// InitialTermios returns the terminal state captured before the child started.
func (s *Session) InitialTermios() (Termios, error) {
	return s.initial, s.initialErr
}

// Termios returns the terminal state of the child's terminal now.
func (s *Session) Termios() (Termios, error) {
	if s.slave == nil {
		return Termios{}, errors.New("ptytest: terminal state needs a pseudo-terminal")
	}
	return readTermios(s.slave)
}

// Close kills the child if it is still running and releases every descriptor.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		s.kill()
		_, _ = s.reap(2 * time.Second)
		s.closeFiles()
	})
}

func (s *Session) closeFiles() {
	// s.input is always the master or the pipe write end, both listed here.
	// Closing an *os.File twice is safe: the second call returns ErrClosed.
	for _, file := range []*os.File{s.resultR, s.stdin, s.stdinR, s.stdoutR, s.stdoutW, s.slave, s.master} {
		if file != nil {
			_ = file.Close()
		}
	}
	s.resultR = nil
}

func (s *Session) kill() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	_ = s.cmd.Process.Kill()
}

// reap waits for the child to exit. It reports false when the timeout expires
// first, leaving the caller to kill the process.
func (s *Session) reap(timeout time.Duration) (bool, error) {
	s.reapMu.Lock()
	defer s.reapMu.Unlock()
	if s.reaped {
		return true, s.waitErr
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-s.waitCh:
		s.reaped, s.waitErr = true, err
		return true, err
	case <-timer.C:
		return false, nil
	}
}

// Output buffers everything a process or a terminal wrote and lets tests wait
// for text without sleeping.
type Output struct {
	mu      sync.Mutex
	data    []byte
	changed chan struct{}
}

// capture starts reading reader in the background and returns its buffer. The
// caller owns reader and must close it to stop the read.
func capture(reader io.Reader) *Output {
	output := &Output{changed: make(chan struct{})}
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				output.mu.Lock()
				output.data = append(output.data, buffer[:n]...)
				close(output.changed)
				output.changed = make(chan struct{})
				output.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	return output
}

// String returns everything captured so far.
func (o *Output) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return string(o.data)
}

// WaitFor waits until the output contains pattern.
func (o *Output) WaitFor(pattern string, timeout time.Duration) (string, bool) {
	return o.WaitForFunc(func(data string) bool {
		return strings.Contains(data, pattern)
	}, timeout)
}

// WaitForFunc waits until match accepts the output. It never sleeps: it wakes
// on every read and on the deadline.
func (o *Output) WaitForFunc(match func(string) bool, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	for {
		o.mu.Lock()
		data := string(o.data)
		changed := o.changed
		o.mu.Unlock()

		if match(data) {
			return data, true
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return data, false
		}
		timer := time.NewTimer(remaining)
		select {
		case <-changed:
			timer.Stop()
		case <-timer.C:
			o.mu.Lock()
			data = string(o.data)
			o.mu.Unlock()
			if match(data) {
				return data, true
			}
			return data, false
		}
	}
}

// Terminal is a pseudo-terminal opened inside the test process, for tests that
// need a terminal-shaped writer without a child process.
type Terminal struct {
	master *os.File
	slave  *os.File
	out    *Output
}

// OpenTerminal opens a pseudo-terminal with a window size and starts capturing
// its master side. It is closed by the test cleanup.
func OpenTerminal(t *testing.T, cols, rows int) *Terminal {
	t.Helper()
	if cols <= 0 {
		cols = defaultCols
	}
	if rows <= 0 {
		rows = defaultRows
	}
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("ptytest: open pseudo-terminal: %v", err)
	}
	size := &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}
	if err := pty.Setsize(slave, size); err != nil {
		_ = master.Close()
		_ = slave.Close()
		t.Fatalf("ptytest: set terminal size: %v", err)
	}
	terminal := &Terminal{master: master, slave: slave, out: capture(master)}
	t.Cleanup(terminal.Close)
	return terminal
}

// Master returns the parent side of the terminal.
func (t *Terminal) Master() *os.File {
	return t.master
}

// Slave returns the side a program writes to.
func (t *Terminal) Slave() *os.File {
	return t.slave
}

// Output returns everything written to the terminal so far.
func (t *Terminal) Output() *Output {
	return t.out
}

// Close releases the terminal.
func (t *Terminal) Close() {
	if t.slave != nil {
		_ = t.slave.Close()
		t.slave = nil
	}
	if t.master != nil {
		_ = t.master.Close()
		t.master = nil
	}
}

// Termios is the part of the terminal state tests assert on.
type Termios struct {
	// Canonical reports whether input is line-buffered (ICANON).
	Canonical bool
	// Echo reports whether typed input is echoed (ECHO).
	Echo bool
	// Signals reports whether control keys generate signals (ISIG).
	Signals bool
}

func (t Termios) String() string {
	return fmt.Sprintf("icanon=%t echo=%t isig=%t", t.Canonical, t.Echo, t.Signals)
}
