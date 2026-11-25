package spinner_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/BeQOC/beqoc-console-ui/spinner"
)

// countingWriter counts every write and can start rejecting them, which is what
// a closed pipe or a vanished terminal does.
type countingWriter struct {
	mu        sync.Mutex
	writes    int
	failAfter int
}

func newCountingWriter(failAfter int) *countingWriter {
	return &countingWriter{failAfter: failAfter}
}

func (w *countingWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes++
	if w.failAfter > 0 && w.writes > w.failAfter {
		return 0, errors.New("the destination is gone")
	}
	return len(data), nil
}

func (w *countingWriter) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writes
}

// waitUntil polls a condition until it holds or the deadline passes.
func waitUntil(condition func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return condition()
}

// TestSpinnerStopsOnAWriterError protects P1-1: a spinner whose destination is
// gone must stop drawing instead of writing into the void forever.
func TestSpinnerStopsOnAWriterError(t *testing.T) {
	writer := newCountingWriter(3)
	progress := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(writer),
		spinner.WithFrames([]string{"."}),
		spinner.WithInterval(5*time.Millisecond),
		spinner.WithAnimation(true),
	).Start()
	defer progress.Stop()

	if !waitUntil(func() bool { return writer.count() > 3 }, 2*time.Second) {
		t.Fatalf("the spinner never reached the failing write; it made %d writes", writer.count())
	}

	// RED: Relatório P1-1 (C4 corrige). Today drawLocked discards the write
	// error and the ticker keeps drawing; C4 ends the loop on a persistent
	// write error. Watching writes, not goroutine counts, keeps this test
	// deterministic: a stopped loop makes no write at all.
	settled := writer.count()
	time.Sleep(200 * time.Millisecond)
	if after := writer.count(); after != settled {
		t.Errorf("RED: Relatório P1-1 (C4 corrige): the spinner wrote %d more frames after the writer failed",
			after-settled)
	}
}

// TestStartIsIdempotent protects the lifecycle: calling Start twice must not
// begin a second animation.
//
// GREEN: mutação "Start() duplicado" (6.1). A second program would draw a
// second initial frame immediately, and it would survive Stop because only the
// newest loop channel is tracked.
func TestStartIsIdempotent(t *testing.T) {
	writer := newCountingWriter(0)
	progress := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(writer),
		spinner.WithFrames([]string{"."}),
		spinner.WithInterval(20*time.Millisecond),
		spinner.WithAnimation(true),
	)
	progress.Start()
	progress.Start()

	// The interval is long enough that no tick can happen between the two calls,
	// so the only write is the first frame.
	if got := writer.count(); got != 1 {
		t.Fatalf("GREEN: mutação Start() duplicado (6.1): %d initial frames, want 1", got)
	}

	progress.Stop()
	settled := writer.count()
	time.Sleep(200 * time.Millisecond)
	if after := writer.count(); after != settled {
		t.Errorf("GREEN: mutação Start() duplicado (6.1): %d frames after Stop, want none", after-settled)
	}
}
