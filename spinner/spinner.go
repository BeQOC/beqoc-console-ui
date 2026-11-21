// Package spinner renders print-style progress feedback: one line, updated in
// place, finished with a single status line.
//
// It is deliberately not a Bubble Tea program. A spinner only needs to animate
// one line and then replace it, so a small frame loop keeps it usable from
// plain commands, scripts and command constructors that never start a
// full-screen interface.
package spinner

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/x/term"

	console "github.com/BeQOC/beqoc-console-ui"
)

// DefaultFrames is the braille frame set.
var DefaultFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// DefaultInterval is the frame interval.
const DefaultInterval = 80 * time.Millisecond

// Option configures a [Spinner].
type Option func(*config)

type config struct {
	theme     *console.Theme
	writer    io.Writer
	frames    []string
	interval  time.Duration
	animation *bool
}

// WithTheme sets the theme used for frames and final messages.
func WithTheme(theme *console.Theme) Option {
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

// Spinner is a single-line progress indicator. Start it, update its text while
// work runs, then finish it with one of Succeed, Fail, Warn or Info.
type Spinner struct {
	theme    *console.Theme
	writer   io.Writer
	frames   []string
	interval time.Duration
	animate  bool

	mu       sync.Mutex
	text     string
	frame    int
	running  bool
	finished bool
	stop     chan struct{}
	done     chan struct{}
}

// New creates a spinner for text.
func New(text string, opts ...Option) *Spinner {
	cfg := config{
		writer:   os.Stdout,
		frames:   DefaultFrames,
		interval: DefaultInterval,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.theme == nil {
		cfg.theme = console.New(console.WithOutput(cfg.writer))
	}
	if cfg.writer == nil {
		cfg.writer = os.Stdout
	}
	if len(cfg.frames) == 0 {
		cfg.frames = DefaultFrames
	}
	if cfg.interval <= 0 {
		cfg.interval = DefaultInterval
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
		text:     text,
	}
}

// Start begins animating. Starting an already running or finished spinner is a
// no-op.
func (s *Spinner) Start() *Spinner {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running || s.finished {
		return s
	}

	s.running = true
	if !s.animate {
		return s
	}

	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.drawLocked()
	go s.loop(s.stop, s.done)
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
		s.drawLocked()
	}
	return s
}

// Stop halts the animation and clears its line. A later finalization still
// prints its final message.
func (s *Spinner) Stop() *Spinner {
	s.mu.Lock()
	if s.finished || !s.running {
		s.mu.Unlock()
		return s
	}
	running, animate := s.running, s.animate
	s.haltLocked()
	stop, done := s.stop, s.done
	s.stop, s.done = nil, nil
	if running && animate {
		s.clearLocked()
	}
	s.mu.Unlock()

	wait(stop, done)
	return s
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
	s.haltLocked()
	stop, done := s.stop, s.done
	s.stop, s.done = nil, nil

	if len(message) > 0 {
		s.text = message[0]
	}
	if wasRunning && animate {
		s.clearLocked()
	}
	line := render(s.text)
	_, _ = io.WriteString(s.writer, line+"\n")
	s.mu.Unlock()

	wait(stop, done)
}

func (s *Spinner) loop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			if s.running && !s.finished {
				s.frame = (s.frame + 1) % len(s.frames)
				s.drawLocked()
			}
			s.mu.Unlock()
		}
	}
}

// haltLocked stops the animation without clearing the line or waiting for the
// loop. The caller must hold s.mu and must call wait after unlocking.
func (s *Spinner) haltLocked() {
	s.running = false
}

func (s *Spinner) drawLocked() {
	frame := s.frames[s.frame%len(s.frames)]
	line := s.theme.Render(s.theme.PrimaryStyle(), frame) + " " +
		s.theme.Render(s.theme.HintStyle(), s.text)
	_, _ = fmt.Fprintf(s.writer, "\r\x1b[2K%s", line)
}

func (s *Spinner) clearLocked() {
	_, _ = io.WriteString(s.writer, "\r\x1b[2K")
}

func wait(stop chan struct{}, done <-chan struct{}) {
	if stop == nil {
		return
	}
	close(stop)
	<-done
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(file.Fd())
}
