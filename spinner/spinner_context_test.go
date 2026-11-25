package spinner_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BeQOC/beqoc-console-ui/spinner"
)

// recordingWriter keeps every byte the spinner writes and is safe to read while
// the frame loop runs.
type recordingWriter struct {
	mu   sync.Mutex
	data strings.Builder
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.data.Write(p)
}

func (w *recordingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.data.String()
}

func (w *recordingWriter) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.data.Len()
}

// TestContextCancellationClearsTheLineAndStopsTheLoop protects the accepted way
// to cancel a spinner: the caller owns the signal and the context, and the
// spinner leaves a clean line with no frame loop behind.
func TestContextCancellationClearsTheLineAndStopsTheLoop(t *testing.T) {
	writer := &recordingWriter{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	progress := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(writer),
		spinner.WithFrames([]string{"."}),
		spinner.WithInterval(5*time.Millisecond),
		spinner.WithAnimation(true),
		spinner.WithContext(ctx),
	).Start()

	if !waitUntil(func() bool { return strings.Count(writer.String(), "\r\x1b[2K") >= 2 }, 2*time.Second) {
		t.Fatalf("the spinner did not animate: %q", writer.String())
	}

	cancel()

	if !waitUntil(func() bool { return strings.HasSuffix(writer.String(), "\r\x1b[2K") }, 2*time.Second) {
		t.Fatalf("the cancellation did not clear the line: %q", writer.String())
	}

	// Nothing may be written after the clear: the ticker goroutine is gone.
	settled := writer.Len()
	time.Sleep(100 * time.Millisecond)
	if after := writer.Len(); after != settled {
		t.Fatalf("the spinner wrote %d bytes after the cancellation", after-settled)
	}

	// Stop reaps the loop; a leaked goroutine would make it hang here.
	stopped := make(chan struct{})
	go func() {
		progress.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after the cancellation; the frame loop leaked")
	}

	// A second Stop and a finalization stay safe and keep the line clean.
	progress.Stop()
	progress.Succeed("done")
	if got := writer.String(); !strings.HasSuffix(got, "✔ done\n") {
		t.Fatalf("output = %q, want the final message on a clean line", got)
	}
}

// TestPreCancelledContextDoesNotAnimate protects the same contract when the
// context is already canceled: nothing is drawn, and the final line still works.
func TestPreCancelledContextDoesNotAnimate(t *testing.T) {
	writer := &recordingWriter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	progress := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(writer),
		spinner.WithFrames([]string{"."}),
		spinner.WithInterval(5*time.Millisecond),
		spinner.WithAnimation(true),
		spinner.WithContext(ctx),
	).Start()
	defer progress.Stop()

	if got := writer.String(); got != "" {
		t.Fatalf("output = %q, want no frame for a canceled context", got)
	}

	progress.Succeed("done")
	if got, want := writer.String(), "✔ done\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

// TestWithWidthTruncatesWithoutATerminal protects the deterministic width used
// by callers and tests that do not write to a real terminal.
func TestWithWidthTruncatesWithoutATerminal(t *testing.T) {
	writer := &recordingWriter{}
	progress := spinner.New("a long text that must be cut",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(writer),
		spinner.WithFrames([]string{"."}),
		spinner.WithInterval(time.Hour),
		spinner.WithAnimation(true),
		spinner.WithWidth(10),
	).Start()
	defer progress.Stop()

	line := strings.TrimPrefix(writer.String(), "\r\x1b[2K")
	if width := len([]rune(line)); width > 10 {
		t.Fatalf("first frame = %q is %d cells wide, want at most 10", line, width)
	}
	if !strings.HasPrefix(line, ". a long") {
		t.Fatalf("first frame = %q, want the frame and the start of the text", line)
	}
}

// TestSpinnerRecordsAWriteError protects the reporting half of the write-error
// contract: the animation stops and the error can be inspected.
func TestSpinnerRecordsAWriteError(t *testing.T) {
	writer := newCountingWriter(2)
	progress := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(writer),
		spinner.WithFrames([]string{"."}),
		spinner.WithInterval(5*time.Millisecond),
		spinner.WithAnimation(true),
	).Start()
	defer progress.Stop()

	if !waitUntil(func() bool { return progress.Err() != nil }, 2*time.Second) {
		t.Fatalf("the spinner did not record the write error; it made %d writes", writer.count())
	}
	if got := progress.Err(); !strings.Contains(got.Error(), "destination is gone") {
		t.Fatalf("Err() = %v, want the writer error", got)
	}
}

// TestASCIIFramesArePrintableASCII keeps the ASCII frame set usable on terminals
// that cannot render the braille frames.
func TestASCIIFramesArePrintableASCII(t *testing.T) {
	frames := spinner.ASCIIFrames()
	if len(frames) < 2 {
		t.Fatalf("ASCIIFrames() returned %d frames, want an animation", len(frames))
	}
	for _, frame := range frames {
		if frame == "" {
			t.Fatalf("ASCIIFrames() contains an empty frame: %q", frames)
		}
		for _, value := range []byte(frame) {
			if value < 0x20 || value >= 0x80 {
				t.Fatalf("frame %q contains a non-printable or non-ASCII byte 0x%02X", frame, value)
			}
		}
	}
}
