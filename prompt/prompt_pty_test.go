//go:build !windows

package prompt_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/BeQOC/beqoc-console-ui/internal/ptytest"
	"github.com/BeQOC/beqoc-console-ui/prompt"
)

// TestPTYHelperProcess is the child entry point of the ptytest harness.
func TestPTYHelperProcess(t *testing.T) {
	ptytest.HelperProcess(t)
}

func ptyPrompter() *prompt.Prompter {
	return prompt.New(prompt.Options{Theme: ptyTheme()})
}

func init() {
	ptytest.RegisterScenario("confirm", func() (any, error) {
		return ptyPrompter().Confirm(context.Background(), prompt.ConfirmOptions{
			Title:       "Continue?",
			Description: "The next step writes files.",
			Affirmative: "Yes",
			Negative:    "No",
		})
	})
	ptytest.RegisterScenario("confirm-default-true", func() (any, error) {
		return ptyPrompter().Confirm(context.Background(), prompt.ConfirmOptions{
			Title:   "Proceed?",
			Default: true,
		})
	})
	ptytest.RegisterScenario("input", func() (any, error) {
		return ptyPrompter().Input(context.Background(), prompt.InputOptions{
			Title:       "Project name",
			Description: "Used in the manifest.",
			Placeholder: "beqoc",
		})
	})
	ptytest.RegisterScenario("input-validate", func() (any, error) {
		return ptyPrompter().Input(context.Background(), prompt.InputOptions{
			Title: "Project name",
			Validate: func(value string) error {
				if value == "bad" {
					return errors.New("name is taken")
				}
				return nil
			},
		})
	})
	ptytest.RegisterScenario("select", func() (any, error) {
		return prompt.Select(context.Background(), ptyPrompter(), prompt.SelectOptions[string]{
			Title:   "Target",
			Choices: targetChoices(),
		})
	})
	ptytest.RegisterScenario("select-default", func() (any, error) {
		beta := "beta"
		return prompt.Select(context.Background(), ptyPrompter(), prompt.SelectOptions[string]{
			Title:   "Target",
			Choices: targetChoices(),
			Default: &beta,
		})
	})
}

func targetChoices() []prompt.Choice[string] {
	return []prompt.Choice[string]{
		{Label: "Alpha", Value: "alpha"},
		{Label: "Beta", Value: "beta"},
		{Label: "Gamma", Value: "gamma"},
	}
}

// startPTY starts a scenario on a real terminal. A zero cols value means the
// default 80x24.
func startPTY(t *testing.T, scenario string, cols int) *ptytest.Session {
	t.Helper()
	return ptytest.Start(t, ptytest.Options{
		Scenario: scenario,
		Cols:     cols,
		Env:      []string{"TERM=xterm-256color"},
	})
}

func resultBool(t *testing.T, result ptytest.Result) bool {
	t.Helper()
	if result.Err != "" {
		t.Fatalf("scenario error = %q", result.Err)
	}
	value, ok := result.Value.(bool)
	if !ok {
		t.Fatalf("scenario value = %#v, want bool", result.Value)
	}
	return value
}

func resultString(t *testing.T, result ptytest.Result) string {
	t.Helper()
	if result.Err != "" {
		t.Fatalf("scenario error = %q", result.Err)
	}
	value, ok := result.Value.(string)
	if !ok {
		t.Fatalf("scenario value = %#v, want string", result.Value)
	}
	return value
}

func TestPTYConfirmAnswers(t *testing.T) {
	t.Run("yes", func(t *testing.T) {
		session := startPTY(t, "confirm", 0)

		session.WaitFor("Continue?")
		session.SendLine("y")
		result := session.Wait()

		if !resultBool(t, result) {
			t.Fatal("confirm = false, want true")
		}
		output := session.Output().String()
		if !strings.Contains(output, "Yes") || !strings.Contains(output, "No") {
			t.Fatalf("prompt does not show the configured answers: %q", output)
		}
	})

	t.Run("no", func(t *testing.T) {
		session := startPTY(t, "confirm", 0)

		session.WaitFor("Continue?")
		session.SendLine("n")
		result := session.Wait()

		if resultBool(t, result) {
			t.Fatal("confirm = true, want false")
		}
	})

	t.Run("default with enter", func(t *testing.T) {
		session := startPTY(t, "confirm-default-true", 0)

		session.WaitFor("Proceed?")
		session.SendLine("")
		result := session.Wait()

		if !resultBool(t, result) {
			t.Fatal("confirm = false, want the default true")
		}
	})
}

func TestPTYInputAcceptsATypedValue(t *testing.T) {
	session := startPTY(t, "input", 0)

	session.WaitFor("Project name")
	session.SendLine("beqoc")
	result := session.Wait()

	if got := resultString(t, result); got != "beqoc" {
		t.Fatalf("input = %q, want beqoc", got)
	}
	if output := session.Output().String(); !strings.Contains(output, "Project name") {
		t.Fatalf("output does not show the field: %q", output)
	}
}

func TestPTYInputRetriesAfterAValidationError(t *testing.T) {
	session := startPTY(t, "input-validate", 0)

	session.WaitFor("Project name")
	session.SendLine("bad")
	session.WaitFor("name is taken")

	// The rejected value stays in the field, so clear it before retrying.
	session.Send("\x7f\x7f\x7f")
	session.SendLine("good")
	result := session.Wait()

	if got := resultString(t, result); got != "good" {
		t.Fatalf("input = %q, want good", got)
	}
	output := session.Output().String()
	if !strings.Contains(output, "name is taken") {
		t.Fatalf("output does not show the validation error: %q", output)
	}
}

func TestPTYSelectMovesToAnIndex(t *testing.T) {
	session := startPTY(t, "select", 0)

	session.WaitFor("Target")
	session.Send("\x1b[B") // down arrow: move from Alpha to Beta
	session.SendLine("")
	result := session.Wait()

	if got := resultString(t, result); got != "beta" {
		t.Fatalf("select = %q, want beta", got)
	}
}

func TestPTYSelectAcceptsAValidDefault(t *testing.T) {
	session := startPTY(t, "select-default", 0)

	session.WaitFor("Target")
	session.SendLine("")
	result := session.Wait()

	if got := resultString(t, result); got != "beta" {
		t.Fatalf("select = %q, want the default beta", got)
	}
}

func TestPTYNarrowTerminalFinishesEveryPrompt(t *testing.T) {
	cases := []struct {
		name     string
		scenario string
		ready    string
		keys     string
		want     string
	}{
		{name: "confirm", scenario: "confirm", ready: "Continue?", keys: "y\r", want: "true"},
		{name: "input", scenario: "input", ready: "Project name", keys: "beqoc\r", want: "beqoc"},
		{name: "select", scenario: "select", ready: "Target", keys: "\x1b[B\r", want: "beta"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			session := startPTY(t, testCase.scenario, 20)

			session.WaitFor(testCase.ready)
			session.Send(testCase.keys)
			result := session.Wait()

			if result.Err != "" {
				t.Fatalf("scenario error = %q", result.Err)
			}
			if got := result.Extra["cols"]; got != "20" {
				t.Fatalf("terminal cols = %q, want 20", got)
			}
			if got := valueString(t, result.Value); got != testCase.want {
				t.Fatalf("value = %q, want %q", got, testCase.want)
			}
			if output := session.Output().String(); !strings.Contains(output, testCase.ready) {
				t.Fatalf("output does not show the field in a narrow terminal: %q", output)
			}
		})
	}
}

func TestPTYResizeDuringAPrompt(t *testing.T) {
	session := startPTY(t, "confirm", 80)

	session.WaitFor("Continue?")
	session.Resize(20, 24)
	session.Resize(40, 24)
	session.SendLine("y")
	result := session.Wait()

	if !resultBool(t, result) {
		t.Fatal("confirm = false, want true")
	}
	if got := result.Extra["cols"]; got != "40" {
		t.Fatalf("terminal cols after resizing = %q, want 40", got)
	}
}

func TestPTYRestoresTheTerminalOnSuccess(t *testing.T) {
	cases := []struct {
		name     string
		scenario string
		ready    string
		keys     string
	}{
		{name: "confirm", scenario: "confirm", ready: "Continue?", keys: "y\r"},
		{name: "input", scenario: "input", ready: "Project name", keys: "beqoc\r"},
		{name: "select", scenario: "select", ready: "Target", keys: "\x1b[B\r"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			session := startPTY(t, testCase.scenario, 0)

			initial, err := session.InitialTermios()
			if err != nil {
				t.Fatalf("initial terminal state: %v", err)
			}
			session.WaitFor(testCase.ready)
			during, err := session.Termios()
			if err != nil {
				t.Fatalf("terminal state during the prompt: %v", err)
			}
			if during.Canonical || during.Echo || during.Signals {
				t.Fatalf("terminal state during the prompt = %v, want raw mode", during)
			}

			session.Send(testCase.keys)
			result := session.Wait()
			if result.Err != "" {
				t.Fatalf("scenario error = %q", result.Err)
			}

			final, err := session.Termios()
			if err != nil {
				t.Fatalf("terminal state: %v", err)
			}
			if final != initial {
				t.Fatalf("terminal state after the prompt = %v, want %v", final, initial)
			}
			if output := session.Output().String(); !strings.Contains(output, "\x1b[?25h") {
				t.Fatalf("the prompt never re-showed the cursor: %q", output)
			}
		})
	}
}

// valueString reads the JSON value of a successful scenario as text, so the
// narrow-terminal table can compare booleans and strings the same way.
func valueString(t *testing.T, value any) string {
	t.Helper()
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		t.Fatalf("scenario value = %#v, want string or bool", value)
		return ""
	}
}
