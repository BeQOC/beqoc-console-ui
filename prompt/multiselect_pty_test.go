//go:build !windows

package prompt_test

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"

	"github.com/BeQOC/beqoc-console-ui/internal/ptytest"
	"github.com/BeQOC/beqoc-console-ui/prompt"
)

func init() {
	// multiselect opens a plain multi-select on the harness terminal.
	ptytest.RegisterScenario("multiselect", scenario(func() (any, error) {
		return prompt.MultiSelect(context.Background(), ptyPrompter(), prompt.MultiSelectOptions[string]{
			Title:   "Targets",
			Choices: multiChoices(),
		})
	}))
	// multiselect-ctx-inflight cancels its own context when the parent resizes
	// the terminal, which proves an in-flight cancellation stops the form.
	ptytest.RegisterScenario("multiselect-ctx-inflight", scenario(func() (any, error) {
		winch := make(chan os.Signal, 1)
		signal.Notify(winch, syscall.SIGWINCH)
		defer signal.Stop(winch)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-winch
			cancel()
		}()
		return prompt.MultiSelect(ctx, ptyPrompter(), prompt.MultiSelectOptions[string]{
			Title:   "Targets",
			Choices: multiChoices(),
		})
	}))
}

// TestPTYMultiSelectTogglesAndSubmits drives the huh multi-select: toggle with
// x, move with j, submit with enter.
func TestPTYMultiSelectTogglesAndSubmits(t *testing.T) {
	session := startFailurePTY(t, "multiselect")

	session.WaitFor("Targets")
	session.Send("x") // toggle Alpha
	session.Send("j") // move to Beta
	session.Send("x") // toggle Beta
	session.SendLine("")

	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)
	if reported.Error != "" {
		t.Fatalf("multi select: %s", reported.Error)
	}
	if got := outcomeStrings(t, reported); !equalStrings(got, []string{"alpha", "beta"}) {
		t.Fatalf("selected = %v, want [alpha beta]", got)
	}

	// The scenario runs with the ProfileNone theme: no color SGR may appear.
	if output := session.Output().String(); strings.Contains(output, "38;") || strings.Contains(output, "48;") {
		t.Fatalf("ProfileNone form emitted a color SGR: %q", output)
	}
}

// TestPTYMultiSelectSearchWithSlash covers the filter the form library offers:
// "/" starts the search, typing narrows it, enter applies and x toggles the
// only match.
func TestPTYMultiSelectSearchWithSlash(t *testing.T) {
	session := startFailurePTY(t, "multiselect")

	session.WaitFor("Targets")
	session.Send("/")
	session.Send("gam")
	session.SendLine("") // apply the filter
	session.Send("x")    // toggle the only visible choice
	session.SendLine("") // submit

	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)
	if reported.Error != "" {
		t.Fatalf("multi select: %s", reported.Error)
	}
	if got := outcomeStrings(t, reported); !equalStrings(got, []string{"gamma"}) {
		t.Fatalf("selected = %v, want [gamma]", got)
	}
}

func TestPTYMultiSelectHonorsACancellationInFlight(t *testing.T) {
	session := startFailurePTY(t, "multiselect-ctx-inflight")

	session.WaitFor("Targets")
	session.Resize(70, 24)

	reported := decodeOutcome(t, session.Wait())
	expectNoPanic(t, reported)
	if !reported.Canceled {
		t.Fatalf("in-flight cancel returned (%v, %q); want context.Canceled", reported.Value, reported.Error)
	}
	if !reported.ErrCanceled {
		t.Fatalf("error %q does not match prompt.ErrCanceled", reported.Error)
	}
}

// outcomeStrings reads the selected values from a scenario outcome.
func outcomeStrings(t *testing.T, reported outcome) []string {
	t.Helper()
	values, ok := reported.Value.([]any)
	if !ok {
		t.Fatalf("scenario value = %#v, want a list of strings", reported.Value)
	}
	got := make([]string, 0, len(values))
	for _, value := range values {
		text, isText := value.(string)
		if !isText {
			t.Fatalf("scenario value = %#v, want a list of strings", reported.Value)
		}
		got = append(got, text)
	}
	return got
}
