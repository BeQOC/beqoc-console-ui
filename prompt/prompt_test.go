package prompt_test

import (
	"bytes"
	"context"
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
	p, out := newPrompter(t, "y\n")

	got, err := p.Confirm(context.Background(), prompt.ConfirmOptions{
		Title:       "Continue?",
		Description: "The next step writes files.",
		Affirmative: "Yes",
		Negative:    "No",
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if !got {
		t.Fatal("confirm returned false, want true")
	}
	if view := out.String(); !strings.Contains(view, "Continue?") || !strings.Contains(view, "[y/N]") {
		t.Fatalf("prompt output = %q", view)
	}
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
	p, _ := newPrompter(t, "\n")

	defaultValue := 30
	got, err := prompt.Select(context.Background(), p, prompt.SelectOptions[int]{
		Title: "Port",
		Choices: []prompt.Choice[int]{
			{Label: "30", Value: 30},
			{Label: "40", Value: 40},
		},
		Default: &defaultValue,
	})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if got != 30 {
		t.Fatalf("select = %d, want 30", got)
	}
}

func TestInputAccessibleFlow(t *testing.T) {
	p, out := newPrompter(t, "beqoc\n")

	got, err := p.Input(context.Background(), prompt.InputOptions{
		Title:       "Project name",
		Description: "Used in the manifest.",
		Placeholder: "beqoc",
		Validate: func(value string) error {
			if value == "" {
				return context.Canceled
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
	if view := out.String(); !strings.Contains(view, "Project name") {
		t.Fatalf("prompt output = %q", view)
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

func TestNilThemeFallsBackToDefault(t *testing.T) {
	styles := prompt.HuhTheme(nil).Theme(true)
	if styles == nil {
		t.Fatal("Theme(nil) returned nil styles")
	}
	p := prompt.New(prompt.Options{})
	if p.Theme() == nil {
		t.Fatal("New used a nil theme")
	}
}
