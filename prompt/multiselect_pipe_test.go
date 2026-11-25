package prompt_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/BeQOC/beqoc-console-ui/prompt"
	"github.com/BeQOC/beqoc-console-ui/theme"
)

// multiChoices is the option list shared by the multi-select tests.
func multiChoices() []prompt.Choice[string] {
	return []prompt.Choice[string]{
		{Label: "Alpha", Value: "alpha"},
		{Label: "Beta", Value: "beta"},
		{Label: "Gamma", Value: "gamma"},
	}
}

func TestMultiSelectLineTogglesByIndexAndValue(t *testing.T) {
	p, out := newPrompter(t, "1\nbeta\ndone\n")

	got, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title:   "Targets",
		Choices: multiChoices(),
	})
	if err != nil {
		t.Fatalf("multi select: %v", err)
	}
	if want := []string{"alpha", "beta"}; !equalStrings(got, want) {
		t.Fatalf("selected = %v, want %v", got, want)
	}

	view := out.String()
	if !strings.Contains(view, "[x] 1. Alpha") || !strings.Contains(view, "[x] 2. Beta") {
		t.Fatalf("prompt output does not mark the selection: %q", view)
	}
	if !strings.Contains(view, "[ ] 3. Gamma") {
		t.Fatalf("prompt output does not keep Gamma unselected: %q", view)
	}
}

func TestMultiSelectLineSearchFiltersTheList(t *testing.T) {
	p, out := newPrompter(t, "?gam\n3\ndone\n")

	got, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title:   "Targets",
		Choices: multiChoices(),
	})
	if err != nil {
		t.Fatalf("multi select: %v", err)
	}
	if want := []string{"gamma"}; !equalStrings(got, want) {
		t.Fatalf("selected = %v, want %v", got, want)
	}

	view := out.String()
	// The initial list shows every choice; the filtered lists show only Gamma.
	if count := strings.Count(view, "Alpha"); count != 1 {
		t.Fatalf("Alpha appears %d times, want once (the filtered list must hide it): %q", count, view)
	}
	if count := strings.Count(view, "Gamma"); count < 3 {
		t.Fatalf("Gamma appears %d times, want the initial and the filtered lists: %q", count, view)
	}
}

func TestMultiSelectLineSearchMatchesValuesToo(t *testing.T) {
	choices := []prompt.Choice[string]{
		{Label: "Prime", Value: "one"},
		{Label: "Second", Value: "two"},
	}
	p, out := newPrompter(t, "?zzz\n?one\n1\ndone\n")

	got, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title:   "Numbers",
		Choices: choices,
	})
	if err != nil {
		t.Fatalf("multi select: %v", err)
	}
	if want := []string{"one"}; !equalStrings(got, want) {
		t.Fatalf("selected = %v, want %v", got, want)
	}
	if view := out.String(); !strings.Contains(view, "no choice matches") {
		t.Fatalf("prompt output does not report a filtered empty list: %q", view)
	}
}

func TestMultiSelectLineRejectsAnUnknownAnswer(t *testing.T) {
	p, out := newPrompter(t, "nope\n1\ndone\n")

	got, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title:   "Targets",
		Choices: multiChoices(),
	})
	if err != nil {
		t.Fatalf("multi select: %v", err)
	}
	if want := []string{"alpha"}; !equalStrings(got, want) {
		t.Fatalf("selected = %v, want %v", got, want)
	}
	if view := out.String(); !strings.Contains(view, `"nope" is not one of the choices`) {
		t.Fatalf("prompt output does not reject the unknown answer: %q", view)
	}
}

func TestMultiSelectLineDeliversInChoiceOrder(t *testing.T) {
	p, _ := newPrompter(t, "3\n1\ndone\n")

	got, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title:   "Targets",
		Choices: multiChoices(),
	})
	if err != nil {
		t.Fatalf("multi select: %v", err)
	}
	if want := []string{"alpha", "gamma"}; !equalStrings(got, want) {
		t.Fatalf("selected = %v, want the choice order %v", got, want)
	}
}

func TestMultiSelectLineNeverRepeatsASelection(t *testing.T) {
	// Beta is toggled on, off and on again; Alpha once.
	p, _ := newPrompter(t, "2\n2\n2\n1\ndone\n")

	got, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title:   "Targets",
		Choices: multiChoices(),
	})
	if err != nil {
		t.Fatalf("multi select: %v", err)
	}
	if want := []string{"alpha", "beta"}; !equalStrings(got, want) {
		t.Fatalf("selected = %v, want %v", got, want)
	}
}

func TestMultiSelectLineRequiresAnExplicitEnd(t *testing.T) {
	p, _ := newPrompter(t, "1\n")

	got, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title:   "Targets",
		Choices: multiChoices(),
	})
	if !errors.Is(err, prompt.ErrEOF) {
		t.Fatalf("err = %v, want prompt.ErrEOF", err)
	}
	if got != nil {
		t.Fatalf("selected = %v, want no partial result at EOF", got)
	}
}

func TestMultiSelectLineHonorsAPreCancelledContext(t *testing.T) {
	p, _ := newPrompter(t, "1\ndone\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := prompt.MultiSelect(ctx, p, prompt.MultiSelectOptions[string]{
		Title:   "Targets",
		Choices: multiChoices(),
	})
	if !errors.Is(err, prompt.ErrCanceled) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want prompt.ErrCanceled and context.Canceled", err)
	}
	if got != nil {
		t.Fatalf("selected = %v, want no result after a cancellation", got)
	}
}

func TestMultiSelectLineHonorsACancellationInFlight(t *testing.T) {
	reader := newGatedReader()
	defer reader.releaseAll()

	p := prompt.New(prompt.Options{
		Theme:      theme.New(theme.WithProfile(theme.ProfileNone)),
		Input:      reader,
		Output:     new(strings.Builder),
		Accessible: true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type answer struct {
		values []string
		err    error
	}
	done := make(chan answer, 1)
	go func() {
		values, err := prompt.MultiSelect(ctx, p, prompt.MultiSelectOptions[string]{
			Title:   "Targets",
			Choices: multiChoices(),
		})
		done <- answer{values: values, err: err}
	}()

	reader.waitForRead(t)
	cancel()

	select {
	case got := <-done:
		if !errors.Is(got.err, prompt.ErrCanceled) || !errors.Is(got.err, context.Canceled) {
			t.Errorf("err = %v, want prompt.ErrCanceled and context.Canceled", got.err)
		}
		if got.values != nil {
			t.Errorf("selected = %v, want no result after a cancellation", got.values)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the prompt did not return after the cancellation")
	}
}

func TestMultiSelectLineAppliesDefaults(t *testing.T) {
	p, out := newPrompter(t, "done\n")

	got, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title:    "Targets",
		Choices:  multiChoices(),
		Defaults: []string{"beta"},
	})
	if err != nil {
		t.Fatalf("multi select: %v", err)
	}
	if want := []string{"beta"}; !equalStrings(got, want) {
		t.Fatalf("selected = %v, want %v", got, want)
	}
	if view := out.String(); !strings.Contains(view, "[x] 2. Beta") {
		t.Fatalf("prompt output does not mark the default: %q", view)
	}
}

func TestMultiSelectRejectsAnEmptyList(t *testing.T) {
	p, _ := newPrompter(t, "done\n")

	got, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title: "Targets",
	})
	if !errors.Is(err, prompt.ErrNoChoices) {
		t.Fatalf("err = %v, want prompt.ErrNoChoices", err)
	}
	if got != nil {
		t.Fatalf("selected = %v, want no result", got)
	}
}

func TestMultiSelectRejectsDefaultsOutsideTheChoices(t *testing.T) {
	p, _ := newPrompter(t, "done\n")

	got, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title:    "Targets",
		Choices:  multiChoices(),
		Defaults: []string{"nope"},
	})
	if !errors.Is(err, prompt.ErrInvalidDefault) {
		t.Fatalf("err = %v, want prompt.ErrInvalidDefault", err)
	}
	if got != nil {
		t.Fatalf("selected = %v, want no result", got)
	}
}

func TestMultiSelectLineEmitsNoEscapesUnderProfileNone(t *testing.T) {
	p, out := newPrompter(t, "?gam\n3\n2\ndone\n")

	if _, err := prompt.MultiSelect(context.Background(), p, prompt.MultiSelectOptions[string]{
		Title:       "Targets",
		Description: "Pick any.",
		Choices:     multiChoices(),
	}); err != nil {
		t.Fatalf("multi select: %v", err)
	}

	if view := out.String(); strings.ContainsRune(view, '\x1b') {
		t.Fatalf("multi-select output under ProfileNone = %q, want no escape sequences", view)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
