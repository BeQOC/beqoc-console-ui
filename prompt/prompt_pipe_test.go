package prompt_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/BeQOC/beqoc-console-ui/prompt"
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
