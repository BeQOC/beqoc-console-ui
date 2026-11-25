package prompt_test

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BeQOC/beqoc-console-ui/prompt"
	"github.com/BeQOC/beqoc-console-ui/theme"
)

// The accessible, line-oriented mode is the supported way to drive prompts from
// a script. The cases here complete what the older tests did not cover: a
// default that is not the first choice and a validator that rejects an answer
// before a later one is accepted.

func TestPipeSelectAppliesADefaultBeyondTheFirstChoice(t *testing.T) {
	p, out := newPrompter(t, "\n")

	beta := "beta"
	got, err := prompt.Select(context.Background(), p, prompt.SelectOptions[string]{
		Title: "Target",
		Choices: []prompt.Choice[string]{
			{Label: "Alpha", Value: "alpha"},
			{Label: "Beta", Value: "beta"},
			{Label: "Gamma", Value: "gamma"},
		},
		Default: &beta,
	})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if got != "beta" {
		t.Fatalf("select = %q, want the default beta", got)
	}
	if view := out.String(); !strings.Contains(view, "2. Beta") {
		t.Fatalf("prompt output does not list Beta: %q", view)
	}
}

func TestPipeInputAppliesValidateBeforeAccepting(t *testing.T) {
	p, out := newPrompter(t, "bad\ngood\n")

	got, err := p.Input(context.Background(), prompt.InputOptions{
		Title: "Project name",
		Validate: func(value string) error {
			if value == "bad" {
				return errors.New("name is taken")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("input: %v", err)
	}
	if got != "good" {
		t.Fatalf("input = %q, want good", got)
	}
	view := out.String()
	if !strings.Contains(view, "name is taken") {
		t.Fatalf("prompt output does not show the validation error: %q", view)
	}
	if count := strings.Count(view, "Project name"); count < 2 {
		t.Fatalf("prompt asked %d times, want a retry after the rejected answer: %q", count, view)
	}
}

// TestPipeSequentialPromptsKeepTheNextAnswer drives two prompts from one pipe.
// The line reader must be shared per Prompter: a fresh scanner per call buffers
// the second answer and then reports EOF, which is what the DevTools had to
// work around with a one-byte reader.
func TestPipeSequentialPromptsKeepTheNextAnswer(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	if _, err := io.WriteString(writer, "one\ntwo\n"); err != nil {
		t.Fatalf("write answers: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close the answer pipe: %v", err)
	}

	p := prompt.New(prompt.Options{
		Theme:      theme.New(theme.WithProfile(theme.ProfileNone)),
		Input:      reader,
		Output:     io.Discard,
		Accessible: true,
	})

	first, err := p.Input(context.Background(), prompt.InputOptions{Title: "First"})
	if err != nil {
		t.Fatalf("first input: %v", err)
	}
	second, err := p.Input(context.Background(), prompt.InputOptions{Title: "Second"})
	if err != nil {
		t.Fatalf("second input: %v", err)
	}

	// RED: C3 (leitor único por Prompter, seção 4.1 do relatório). Today the
	// first call's scanner holds "two\n" in its buffer and discards it, so the
	// second call sees EOF and returns an empty value with a nil error.
	if first != "one" {
		t.Errorf("first input = %q, want one", first)
	}
	if second != "two" {
		t.Errorf("second input = %q, want two; the first prompt swallowed the buffered answer", second)
	}
}

// TestPipeSequenceOfPromptsKeepsEveryAnswer drives two inputs and a select from
// one pipe. Each call must consume exactly its own line: this is the end-to-end
// protection of the shared reader.
func TestPipeSequenceOfPromptsKeepsEveryAnswer(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	if _, err := io.WriteString(writer, "one\ntwo\n3\n"); err != nil {
		t.Fatalf("write answers: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close the answer pipe: %v", err)
	}

	p := prompt.New(prompt.Options{
		Theme:      theme.New(theme.WithProfile(theme.ProfileNone)),
		Input:      reader,
		Output:     io.Discard,
		Accessible: true,
	})

	first, err := p.Input(context.Background(), prompt.InputOptions{Title: "First"})
	if err != nil {
		t.Fatalf("first input: %v", err)
	}
	second, err := p.Input(context.Background(), prompt.InputOptions{Title: "Second"})
	if err != nil {
		t.Fatalf("second input: %v", err)
	}
	third, err := prompt.Select(context.Background(), p, prompt.SelectOptions[string]{
		Title: "Third",
		Choices: []prompt.Choice[string]{
			{Label: "Alpha", Value: "alpha"},
			{Label: "Beta", Value: "beta"},
			{Label: "Gamma", Value: "gamma"},
		},
	})
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	if first != "one" || second != "two" || third != "gamma" {
		t.Fatalf("answers = (%q, %q, %q), want (one, two, gamma)", first, second, third)
	}
}

// TestPrompterSerializesConcurrentCalls documents the concurrency policy: a
// Prompter is meant for one conversation at a time, and the internal mutex keeps
// the shared reader coherent if callers use it from several goroutines anyway.
func TestPrompterSerializesConcurrentCalls(t *testing.T) {
	p := prompt.New(prompt.Options{
		Theme:      theme.New(theme.WithProfile(theme.ProfileNone)),
		Input:      strings.NewReader("one\ntwo\n"),
		Output:     io.Discard,
		Accessible: true,
	})

	answers := make(chan string, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			value, err := p.Input(context.Background(), prompt.InputOptions{Title: "Name"})
			if err != nil {
				t.Errorf("input: %v", err)
				return
			}
			answers <- value
		}()
	}
	group.Wait()
	close(answers)

	got := make([]string, 0, 2)
	for answer := range answers {
		got = append(got, answer)
	}
	slices.Sort(got)
	if want := []string{"one", "two"}; !slices.Equal(got, want) {
		t.Fatalf("answers = %v, want %v (each call must keep its own line)", got, want)
	}
}

// gatedReader blocks every read until the test releases it, which makes an
// in-flight read observable and keeps the ordering deterministic. Each answer
// is served on its own release; when the answers run out the reader either
// blocks once more (blockOnEOF) or reports EOF immediately.
type gatedReader struct {
	mu         sync.Mutex
	answers    [][]byte
	index      int
	blockOnEOF bool

	started  chan struct{}
	releases chan struct{}
}

func newGatedReader(answers ...string) *gatedReader {
	reader := &gatedReader{
		blockOnEOF: true,
		started:    make(chan struct{}, 16),
		releases:   make(chan struct{}, 16),
	}
	for _, answer := range answers {
		reader.answers = append(reader.answers, []byte(answer))
	}
	return reader
}

func (r *gatedReader) Read(buffer []byte) (int, error) {
	r.started <- struct{}{}

	r.mu.Lock()
	drained := r.index >= len(r.answers)
	r.mu.Unlock()
	if drained && !r.blockOnEOF {
		return 0, io.EOF
	}

	<-r.releases

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.index >= len(r.answers) {
		return 0, io.EOF
	}
	answer := r.answers[r.index]
	r.index++
	return copy(buffer, answer), nil
}

// waitForRead waits until the prompt is blocked on a read.
func (r *gatedReader) waitForRead(t *testing.T) {
	t.Helper()
	select {
	case <-r.started:
	case <-time.After(2 * time.Second):
		t.Fatal("the prompt never started reading")
	}
}

// release lets one blocked read complete.
func (r *gatedReader) release() {
	r.releases <- struct{}{}
}

// releaseAll frees any read still blocked at the end of a test.
func (r *gatedReader) releaseAll() {
	for index := 0; index < cap(r.releases); index++ {
		select {
		case r.releases <- struct{}{}:
		default:
			return
		}
	}
}

// inputWithin fails the test instead of blocking when a prompt never answers.
func inputWithin(t *testing.T, p *prompt.Prompter, timeout time.Duration) (string, error) {
	t.Helper()
	type answer struct {
		value string
		err   error
	}
	done := make(chan answer, 1)
	go func() {
		value, err := p.Input(context.Background(), prompt.InputOptions{Title: "Name"})
		done <- answer{value: value, err: err}
	}()
	select {
	case got := <-done:
		return got.value, got.err
	case <-time.After(timeout):
		t.Fatal("the prompt did not answer; the pending answer was lost or duplicated")
		return "", nil
	}
}

// TestLineModeCancelsAnInFlightRead proves that a cancellation arriving while a
// line prompt waits for input ends the prompt with an error, not with its
// default.
func TestLineModeCancelsAnInFlightRead(t *testing.T) {
	reader := newGatedReader()
	defer reader.releaseAll()

	p := prompt.New(prompt.Options{
		Theme:      theme.New(theme.WithProfile(theme.ProfileNone)),
		Input:      reader,
		Output:     io.Discard,
		Accessible: true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type answer struct {
		value bool
		err   error
	}
	done := make(chan answer, 1)
	go func() {
		value, err := p.Confirm(ctx, prompt.ConfirmOptions{Title: "Continue?", Default: true})
		done <- answer{value: value, err: err}
	}()

	reader.waitForRead(t)
	cancel()

	select {
	case got := <-done:
		if !errors.Is(got.err, prompt.ErrCanceled) {
			t.Errorf("err = %v, want prompt.ErrCanceled", got.err)
		}
		if !errors.Is(got.err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", got.err)
		}
		if got.value {
			t.Errorf("value = true; a cancellation must not return an answer")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the prompt did not return after the cancellation")
	}
}

// TestLineModeKeepsAnAnswerThatArrivesAfterACancellation proves that the read
// which was in flight when the prompt was canceled keeps its answer for the next
// prompt, and that the answer is delivered exactly once.
func TestLineModeKeepsAnAnswerThatArrivesAfterACancellation(t *testing.T) {
	reader := newGatedReader("typed\n")
	reader.blockOnEOF = false
	defer reader.releaseAll()

	p := prompt.New(prompt.Options{
		Theme:      theme.New(theme.WithProfile(theme.ProfileNone)),
		Input:      reader,
		Output:     io.Discard,
		Accessible: true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type answer struct {
		value string
		err   error
	}
	interrupted := make(chan answer, 1)
	go func() {
		value, err := p.Input(ctx, prompt.InputOptions{Title: "Name"})
		interrupted <- answer{value: value, err: err}
	}()

	reader.waitForRead(t)
	cancel()

	select {
	case got := <-interrupted:
		if !errors.Is(got.err, prompt.ErrCanceled) {
			t.Fatalf("interrupted err = %v, want prompt.ErrCanceled", got.err)
		}
		if got.value != "" {
			t.Fatalf("interrupted value = %q, want no answer", got.value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the interrupted prompt did not return")
	}

	// The answer was already on its way when the prompt was canceled; the read
	// completes now and belongs to the next call, not to the canceled one.
	reader.release()

	value, err := inputWithin(t, p, 2*time.Second)
	if err != nil {
		t.Fatalf("next input: %v (the pending answer was lost)", err)
	}
	if value != "typed" {
		t.Fatalf("next input = %q, want typed", value)
	}

	// The answer must not be delivered twice: the stream is drained now.
	if _, err := inputWithin(t, p, 2*time.Second); !errors.Is(err, prompt.ErrEOF) {
		t.Fatalf("third input err = %v, want prompt.ErrEOF (the answer was repeated)", err)
	}
}

// TestLineModeCancelsAnInFlightReadAtItsDeadline covers the deadline flavor of
// the same cancellation path.
func TestLineModeCancelsAnInFlightReadAtItsDeadline(t *testing.T) {
	reader := newGatedReader()
	defer reader.releaseAll()

	p := prompt.New(prompt.Options{
		Theme:      theme.New(theme.WithProfile(theme.ProfileNone)),
		Input:      reader,
		Output:     io.Discard,
		Accessible: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	value, err := p.Input(ctx, prompt.InputOptions{Title: "Name"})
	if !errors.Is(err, prompt.ErrCanceled) {
		t.Errorf("err = %v, want prompt.ErrCanceled", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
	if value != "" {
		t.Errorf("value = %q; a deadline must not return an answer", value)
	}
}
