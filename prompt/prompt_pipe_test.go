package prompt_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

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
