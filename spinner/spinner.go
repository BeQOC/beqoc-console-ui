// Package spinner renders print-style progress feedback: one line, updated in
// place, finished with a single status line.
//
// It is deliberately not a Bubble Tea program. A spinner only needs to animate
// one line and then replace it, so a small frame loop keeps it usable from
// plain commands, scripts and command constructors that never start a
// full-screen interface.
//
// Every frame is truncated to the current terminal width, read before each
// draw, so a terminal resize is followed on the next frame without a signal
// listener of its own. A write error stops the animation and is reported by
// [Spinner.Err].
//
// A spinner never installs signal handlers and never decides what a signal
// means: cancellation belongs to the caller. Derive a context from the signal
// handler you own, pass it with [WithContext], and the spinner clears its line
// and stops its frame loop when the context ends.
package spinner

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"

	"github.com/BeQOC/beqoc-console-ui/theme"
)

// DefaultFrames returns a copy of the braille frame set.
func DefaultFrames() []string {
	return []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
}

// ASCIIFrames returns a frame set that only uses printable ASCII, for terminals
// that cannot show the braille frames.
func ASCIIFrames() []string {
	return []string{"|", "/", "-", "\\"}
}

// DefaultInterval is the frame interval.
const DefaultInterval = 80 * time.Millisecond

// Option configures a [Spinner].
type Option func(*config)

type config struct {
	theme     *theme.Theme
	writer    io.Writer
	frames    []string
	interval  time.Duration
	animation *bool
	width     int
	ctx       context.Context
}

// WithTheme sets the theme used for frames and final messages.
func WithTheme(theme *theme.Theme) Option {
	return func(c *config) {
		if theme != nil {
			c.theme = theme
		}
	}
}

// WithWriter sets the output stream.
func WithWriter(w io.Writer) Option {
	return func(c *config) {
		if w != nil {
			c.writer = w
		}
	}
}

// WithFrames replaces the animation frames. Empty frames are ignored.
func WithFrames(frames []string) Option {
	return func(c *config) {
		if len(frames) > 0 {
			c.frames = append([]string(nil), frames...)
		}
	}
}

// WithInterval sets the frame interval. Non-positive values are ignored.
func WithInterval(interval time.Duration) Option {
	return func(c *config) {
		if interval > 0 {
			c.interval = interval
		}
	}
}

// WithAnimation forces animation on or off. By default a spinner animates only
// when its writer is a terminal whose TERM is not "dumb".
func WithAnimation(enabled bool) Option {
	return func(c *config) {
		c.animation = &enabled
	}
}

// WithWidth fixes the line width used for truncation. By default the spinner
// reads the terminal width before every frame; use this option for
// deterministic output when the writer is not a terminal.
func WithWidth(width int) Option {
	return func(c *config) {
		if width > 0 {
			c.width = width
		}
	}
}

// WithContext cancels the animation when ctx ends: the line is cleared and the
// frame loop stops, like [Spinner.Stop]. The spinner never installs signal
// handlers, so cancellation is the caller's decision.
func WithContext(ctx context.Context) Option {
	return func(c *config) {
		if ctx != nil {
			c.ctx = ctx
		}
	}
}

// Spinner is a single-line progress indicator. Start it, update its text while
// work runs, then finish it with one of Succeed, Fail, Warn or Info.
type Spinner struct {
	theme    *theme.Theme
	writer   io.Writer
	frames   []string
	interval time.Duration
	animate  bool
	width    int
	ctx      context.Context

	mu       sync.Mutex
	text     string
	frame    int
	running  bool
	finished bool
	cancel   context.CancelFunc
	done     chan struct{}
	writeErr error
}

// New creates a spinner for text.
func New(text string, opts ...Option) *Spinner {
	cfg := config{
		writer:   os.Stdout,
		frames:   DefaultFrames(),
		interval: DefaultInterval,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.theme == nil {
		cfg.theme = theme.New(theme.WithOutput(cfg.writer))
	}
	animate := isTerminal(cfg.writer) && os.Getenv("TERM") != "dumb"
	if cfg.animation != nil {
		animate = *cfg.animation
	}

	return &Spinner{
		theme:    cfg.theme,
		writer:   cfg.writer,
		frames:   cfg.frames,
		interval: cfg.interval,
		animate:  animate,
		width:    cfg.width,
		ctx:      cfg.ctx,
		text:     text,
	}
}

// Start begins animating. Starting an already running or finished spinner is a
// no-op, and so is starting a spinner whose context is already canceled.
func (s *Spinner) Start() *Spinner {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running || s.finished {
		return s
	}
	if s.ctx != nil && s.ctx.Err() != nil {
		return s
	}

	s.running = true
	if !s.animate {
		return s
	}

	parent := s.ctx
	if parent == nil {
		parent = context.Background()
	}
	runCtx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.done = make(chan struct{})
	if err := s.drawLocked(); err != nil {
		// The first frame failed: the loop starts only to notice the
		// cancellation and finish.
		cancel()
	}
	go s.loop(runCtx, s.done)
	return s
}

// Update replaces the spinner text and redraws it when animating.
func (s *Spinner) Update(text string) *Spinner {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return s
	}
	s.text = text
	if s.running && s.animate {
		if err := s.drawLocked(); err != nil && s.cancel != nil {
			s.cancel()
		}
	}
	return s
}

// Stop halts the animation and clears its line. A later finalization still
// prints its final message.
func (s *Spinner) Stop() *Spinner {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return s
	}
	wasRunning := s.running
	s.running = false
	if wasRunning && s.animate {
		s.clearLocked()
	}
	// A loop may still exist after a context cancellation or a write error;
	// Stop always reaps it, so stopping never leaves the frame loop behind.
	cancel, done := s.cancel, s.done
	s.cancel, s.done = nil, nil
	s.mu.Unlock()

	stopLoop(cancel, done)
	return s
}

// Err reports the first write error that stopped the animation, if any.
func (s *Spinner) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeErr
}

// Succeed finishes with a success line. An optional message replaces the
// current text.
func (s *Spinner) Succeed(message ...string) *Spinner {
	s.finish(s.theme.Success, message...)
	return s
}

// Fail finishes with an error line. An optional message replaces the current
// text.
func (s *Spinner) Fail(message ...string) *Spinner {
	s.finish(s.theme.Error, message...)
	return s
}

// Warn finishes with a warning line. An optional message replaces the current
// text.
func (s *Spinner) Warn(message ...string) *Spinner {
	s.finish(s.theme.Warn, message...)
	return s
}

// Info finishes with an info line. An optional message replaces the current
// text.
func (s *Spinner) Info(message ...string) *Spinner {
	s.finish(s.theme.Info, message...)
	return s
}

func (s *Spinner) finish(render func(string) string, message ...string) {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.finished = true

	wasRunning, animate := s.running, s.animate
	s.running = false
	cancel, done := s.cancel, s.done
	s.cancel, s.done = nil, nil

	if len(message) > 0 {
		s.text = message[0]
	}
	if wasRunning && animate {
		s.clearLocked()
	}
	line := render(s.text)
	_, _ = io.WriteString(s.writer, line+"\n")
	s.mu.Unlock()

	stopLoop(cancel, done)
}

func (s *Spinner) loop(ctx context.Context, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// A cancellation from the caller: leave a clean line behind. Stop
			// and finish set running first, so they never clear twice.
			s.mu.Lock()
			if s.running {
				s.running = false
				if s.animate {
					s.clearLocked()
				}
			}
			s.mu.Unlock()
			return
		case <-ticker.C:
			s.mu.Lock()
			if s.running && !s.finished {
				s.frame = (s.frame + 1) % len(s.frames)
				if s.drawLocked() != nil {
					s.mu.Unlock()
					return
				}
			}
			s.mu.Unlock()
		}
	}
}

// drawLocked renders and writes one frame. A write error stops the animation
// and is recorded; the caller decides whether to end the loop.
func (s *Spinner) drawLocked() error {
	frame := s.frames[s.frame%len(s.frames)]
	line := s.theme.Render(s.theme.PrimaryStyle(), frame) + " " +
		s.theme.Render(s.theme.HintStyle(), s.text)
	if width := s.widthLocked(); width > 0 {
		line = ansi.Truncate(line, width, "")
	}
	if _, err := fmt.Fprintf(s.writer, "\r\x1b[2K%s", line); err != nil {
		if s.writeErr == nil {
			s.writeErr = err
		}
		s.running = false
		return err
	}
	return nil
}

// widthLocked reports the width to truncate to, or 0 when it is unknown. The
// terminal is asked before every draw, which is what makes a resize visible on
// the next frame without a SIGWINCH listener.
func (s *Spinner) widthLocked() int {
	if s.width > 0 {
		return s.width
	}
	file, ok := s.writer.(*os.File)
	if !ok || !term.IsTerminal(file.Fd()) {
		return 0
	}
	width, _, err := term.GetSize(file.Fd())
	if err != nil {
		return 0
	}
	return width
}

func (s *Spinner) clearLocked() {
	_, _ = io.WriteString(s.writer, "\r\x1b[2K")
}

// stopLoop cancels the frame loop and waits for it to end. Canceling is
// idempotent, so stopping twice is safe.
func stopLoop(cancel context.CancelFunc, done <-chan struct{}) {
	if cancel == nil {
		return
	}
	cancel()
	if done != nil {
		<-done
	}
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(file.Fd())
}
