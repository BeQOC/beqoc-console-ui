package prompt_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/BeQOC/beqoc-console-ui/prompt"
	"github.com/BeQOC/beqoc-console-ui/theme"
)

func newPrompter(t *testing.T, input string) (*prompt.Prompter, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	p := prompt.New(prompt.Options{
		Theme:      theme.New(theme.WithProfile(theme.ProfileNone)),
		Input:      strings.NewReader(input),
		Output:     &out,
		Accessible: true,
	})
	return p, &out
}

func TestConfirmAccessibleFlow(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "affirmative", input: "y\n", want: true},
		{name: "negative", input: "n\n", want: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			p, out := newPrompter(t, testCase.input)

			got, err := p.Confirm(context.Background(), prompt.ConfirmOptions{
				Title:       "Continue?",
				Description: "The next step writes files.",
				Affirmative: "Yes",
				Negative:    "No",
			})
			if err != nil {
				t.Fatalf("confirm: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("confirm = %t, want %t", got, testCase.want)
			}

			// The accessible renderer prints the title and the default indicator
			// only: huh does not render Description, Affirmative or Negative in
			// this mode, so dropping those options is not observable here.
			// TestPTYConfirmRendersDescriptionAndAnswerLabels asserts them where
			// huh does render them, and fails when any of them is dropped.
			view := out.String()
			if !strings.Contains(view, "Continue?") || !strings.Contains(view, "[y/N]") {
				t.Fatalf("prompt output = %q", view)
			}
		})
	}

	t.Run("default is reflected in the indicator", func(t *testing.T) {
		p, out := newPrompter(t, "\n")
		if _, err := p.Confirm(context.Background(), prompt.ConfirmOptions{Title: "Proceed?", Default: true}); err != nil {
			t.Fatalf("confirm: %v", err)
		}
		if view := out.String(); !strings.Contains(view, "[Y/n]") {
			t.Fatalf("prompt output = %q, want the yes-default indicator", view)
		}
	})
}

func TestConfirmNegativeAnswer(t *testing.T) {
	p, _ := newPrompter(t, "n\n")
	got, err := p.Confirm(context.Background(), prompt.ConfirmOptions{Title: "Continue?"})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if got {
		t.Fatal("confirm returned true, want false")
	}
}

func TestConfirmDefaultAppliesToAnEmptyAnswer(t *testing.T) {
	p, _ := newPrompter(t, "\n")
	got, err := p.Confirm(context.Background(), prompt.ConfirmOptions{Title: "Proceed?", Default: true})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if !got {
		t.Fatal("confirm returned false, want the default true")
	}
}

func TestSelectAccessibleFlow(t *testing.T) {
	p, out := newPrompter(t, "2\n")

	got, err := prompt.Select(context.Background(), p, prompt.SelectOptions[string]{
		Title: "Target",
		Choices: []prompt.Choice[string]{
			{Label: "Alpha", Value: "alpha"},
			{Label: "Beta", Value: "beta"},
			{Label: "Gamma", Value: "gamma"},
		},
	})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if got != "beta" {
		t.Fatalf("select = %q, want beta", got)
	}
	view := out.String()
	if !strings.Contains(view, "Target") || !strings.Contains(view, "2. Beta") {
		t.Fatalf("prompt output = %q", view)
	}
}

func TestSelectDefaultAndTypedValues(t *testing.T) {
	choices := []prompt.Choice[int]{
		{Label: "30", Value: 30},
		{Label: "40", Value: 40},
		{Label: "50", Value: 50},
	}

	t.Run("default is not the first choice", func(t *testing.T) {
		p, _ := newPrompter(t, "\n")

		defaultValue := 40
		got, err := prompt.Select(context.Background(), p, prompt.SelectOptions[int]{
			Title:   "Port",
			Choices: choices,
			Default: &defaultValue,
		})
		if err != nil {
			t.Fatalf("select: %v", err)
		}
		if got != 40 {
			t.Fatalf("select = %d, want the default 40", got)
		}
	})

	t.Run("typed index wins over the default", func(t *testing.T) {
		p, _ := newPrompter(t, "3\n")

		defaultValue := 40
		got, err := prompt.Select(context.Background(), p, prompt.SelectOptions[int]{
			Title:   "Port",
			Choices: choices,
			Default: &defaultValue,
		})
		if err != nil {
			t.Fatalf("select: %v", err)
		}
		if got != 50 {
			t.Fatalf("select = %d, want the typed choice 50", got)
		}
	})
}

func TestInputAccessibleFlow(t *testing.T) {
	p, out := newPrompter(t, "taken\nbeqoc\n")

	got, err := p.Input(context.Background(), prompt.InputOptions{
		Title:       "Project name",
		Description: "Used in the manifest.",
		Placeholder: "beqoc",
		Validate: func(value string) error {
			if value == "taken" {
				return errors.New("that name is taken")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("input: %v", err)
	}
	if got != "beqoc" {
		t.Fatalf("input = %q, want beqoc", got)
	}

	view := out.String()
	if !strings.Contains(view, "Project name") {
		t.Fatalf("prompt output does not show the field: %q", view)
	}
	// The validator must run against the first answer and the prompt must ask
	// again instead of accepting it.
	if !strings.Contains(view, "that name is taken") {
		t.Fatalf("prompt output does not show the validation error: %q", view)
	}
	if count := strings.Count(view, "Project name"); count < 2 {
		t.Fatalf("prompt asked %d times, want a retry after the rejected answer: %q", count, view)
	}
	// Placeholder and Description are not printed by huh's accessible renderer,
	// so dropping them is not observable here; TestPTYInputRendersThePlaceholder
	// and TestPTYConfirmRendersDescriptionAndAnswerLabels assert them where huh
	// does render them.
}

func TestNilThemeFallsBackToDefault(t *testing.T) {
	fallback := prompt.HuhTheme(nil).Theme(true)
	explicit := prompt.HuhTheme(theme.New()).Theme(true)
	if fallback == nil || explicit == nil {
		t.Fatal("Theme returned nil styles")
	}

	// A nil theme must build the same styles as an explicitly built default
	// theme. Returning huh's generic base theme instead fails every comparison.
	fields := []struct {
		name string
		got  any
		want any
	}{
		{"Focused.Title", fallback.Focused.Title, explicit.Focused.Title},
		{"Focused.Description", fallback.Focused.Description, explicit.Focused.Description},
		{"Focused.ErrorMessage", fallback.Focused.ErrorMessage, explicit.Focused.ErrorMessage},
		{"Focused.SelectSelector", fallback.Focused.SelectSelector, explicit.Focused.SelectSelector},
		{"Focused.SelectedPrefix", fallback.Focused.SelectedPrefix, explicit.Focused.SelectedPrefix},
		{"Focused.FocusedButton", fallback.Focused.FocusedButton, explicit.Focused.FocusedButton},
		{"Focused.TextInput.Placeholder", fallback.Focused.TextInput.Placeholder, explicit.Focused.TextInput.Placeholder},
		{"Help.ShortDesc", fallback.Help.ShortDesc, explicit.Help.ShortDesc},
	}
	for _, field := range fields {
		if !reflect.DeepEqual(field.got, field.want) {
			t.Errorf("%s = %v, want the default theme's %v", field.name, field.got, field.want)
		}
	}

	p := prompt.New(prompt.Options{})
	if p.Theme() == nil {
		t.Fatal("New used a nil theme")
	}
}

func TestInputDefaultAppliesToAnEmptyAnswer(t *testing.T) {
	p, _ := newPrompter(t, "\n")
	got, err := p.Input(context.Background(), prompt.InputOptions{Title: "Name", Default: "default-name"})
	if err != nil {
		t.Fatalf("input: %v", err)
	}
	if got != "default-name" {
		t.Fatalf("input = %q, want default-name", got)
	}
}

func TestThemeUsesConsoleColors(t *testing.T) {
	theme := theme.New(theme.WithProfile(theme.ProfileTrueColor), theme.WithDark(true))

	styles := prompt.HuhTheme(theme).Theme(false)
	if got, want := styles.Focused.Title.GetForeground(), theme.Styles().Primary.GetForeground(); !reflect.DeepEqual(got, want) {
		t.Errorf("title foreground = %v, want %v", got, want)
	}
	if got, want := styles.Focused.ErrorMessage.GetForeground(), theme.Styles().Error.GetForeground(); !reflect.DeepEqual(got, want) {
		t.Errorf("error foreground = %v, want %v", got, want)
	}
	if got, want := styles.Focused.Description.GetForeground(), theme.Styles().Hint.GetForeground(); !reflect.DeepEqual(got, want) {
		t.Errorf("description foreground = %v, want %v", got, want)
	}
}
