package prompt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
)

// errTerminalGone reports a terminal that closed while a form was open. It
// deliberately does not wrap io.EOF: bubbletea treats an end of input as a
// clean stop of its reader and then waits forever for a message that never
// arrives.
var errTerminalGone = errors.New("prompt: the terminal closed")

// cancelCause remembers the first reason a form was interrupted, so the prompt
// can tell a signal, Ctrl-D, a vanished terminal and a context cancellation
// apart after the form library has already collapsed them.
type cancelCause struct {
	mu  sync.Mutex
	err error
}

func (c *cancelCause) set(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		c.err = err
	}
}

func (c *cancelCause) get() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// terminalInput watches the terminal the form reads from. Bubble Tea swallows a
// read failure or an end of input, so the wrapper is the only place where a
// hangup is visible: it cancels the form instead of letting it wait forever.
type terminalInput struct {
	file  term.File
	onEnd func()
}

func (t *terminalInput) Read(buffer []byte) (int, error) {
	n, err := t.file.Read(buffer)
	if n == 0 && isTerminalGone(err) {
		t.onEnd()
		return 0, errTerminalGone
	}
	return n, err
}

func (t *terminalInput) Write(buffer []byte) (int, error) { return t.file.Write(buffer) }

func (t *terminalInput) Close() error { return t.file.Close() }

func (t *terminalInput) Fd() uintptr { return t.file.Fd() }

// promptKeyFilter turns Ctrl-D into an interrupt while remembering that the end
// of input, not Ctrl-C, ended the question.
func promptKeyFilter(cause *cancelCause) func(tea.Model, tea.Msg) tea.Msg {
	return func(_ tea.Model, msg tea.Msg) tea.Msg {
		key, ok := msg.(tea.KeyPressMsg)
		if !ok || key.String() != "ctrl+d" {
			return msg
		}
		cause.set(ErrEOF)
		return tea.InterruptMsg{}
	}
}

// watchSignals cancels the form on SIGINT or SIGTERM and records why. Bubble
// Tea's own handler is disabled for the form, so a signal becomes a prompt error
// instead of a silent success; the caller still owns the process.
func watchSignals(ctx context.Context, cancel context.CancelFunc, cause *cancelCause) func() {
	signals := make(chan os.Signal, 1)
	notifySignals(signals)
	done := make(chan struct{})

	go func() {
		select {
		case received := <-signals:
			cause.set(fmt.Errorf("%w: received %s", ErrCanceled, received))
			cancel()
		case <-done:
		case <-ctx.Done():
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			signal.Stop(signals)
			close(done)
		})
	}
}

// runForm runs the huh form on a real terminal.
func (p *Prompter) runForm(ctx context.Context, field huh.Field) error {
	if err := ctx.Err(); err != nil {
		return cancelError(err)
	}

	cause := &cancelCause{}
	formCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stopSignals := watchSignals(formCtx, cancel, cause)
	defer stopSignals()

	input := p.input
	if file, ok := p.input.(term.File); ok {
		input = &terminalInput{
			file: file,
			onEnd: func() {
				cause.set(fmt.Errorf("%w: the terminal closed", ErrCanceled))
				cancel()
			},
		}
	}

	// WithProgramOptions replaces huh's option list, so the streams are wired
	// after it or they would be dropped.
	form := huh.NewForm(huh.NewGroup(field)).
		WithTheme(huhTheme(p.theme)).
		WithProgramOptions(
			tea.WithoutSignalHandler(),
			tea.WithFilter(promptKeyFilter(cause)),
		).
		WithInput(input).
		WithOutput(p.output)

	err := runFormSafely(form, formCtx)

	if reason := cause.get(); reason != nil {
		return reason
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return cancelError(ctxErr)
	}
	switch {
	case err == nil:
		return nil
	case errors.Is(err, huh.ErrUserAborted):
		return fmt.Errorf("%w: interrupted", ErrCanceled)
	case errors.Is(err, huh.ErrTimeout):
		return fmt.Errorf("%w: the form stopped", ErrCanceled)
	default:
		return fmt.Errorf("prompt: form: %w", err)
	}
}

// runFormSafely runs the form and turns a panic into an error. huh panics when
// bubbletea hands it a nil model, which used to be a second uncaught panic on a
// non-terminal input; the caller now gets an error instead.
func runFormSafely(form *huh.Form, ctx context.Context) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("prompt: the form failed: %v", recovered)
		}
	}()
	return form.RunWithContext(ctx)
}
