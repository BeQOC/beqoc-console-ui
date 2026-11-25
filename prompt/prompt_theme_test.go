package prompt_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/BeQOC/beqoc-console-ui/prompt"
	themeapi "github.com/BeQOC/beqoc-console-ui/theme"
)

// accessibleCase is one line-oriented prompt call and the answer it consumes.
type accessibleCase struct {
	name  string
	input string
	run   func(*prompt.Prompter) error
}

func accessibleCases() []accessibleCase {
	return []accessibleCase{
		{name: "confirm", input: "y\n", run: func(prompter *prompt.Prompter) error {
			_, err := prompter.Confirm(context.Background(), prompt.ConfirmOptions{
				Title:       "Continue?",
				Description: "The next step writes files.",
				Affirmative: "Yes",
				Negative:    "No",
			})
			return err
		}},
		{name: "input", input: "beqoc\n", run: func(prompter *prompt.Prompter) error {
			_, err := prompter.Input(context.Background(), prompt.InputOptions{
				Title:       "Project name",
				Description: "Used in the manifest.",
				Placeholder: "beqoc",
			})
			return err
		}},
		{name: "select", input: "2\n", run: func(prompter *prompt.Prompter) error {
			_, err := prompt.Select(context.Background(), prompter, prompt.SelectOptions[string]{
				Title: "Target",
				Choices: []prompt.Choice[string]{
					{Label: "Alpha", Value: "alpha"},
					{Label: "Beta", Value: "beta"},
				},
			})
			return err
		}},
	}
}

// accessibleOutput runs a prompt in line mode and returns everything it wrote.
// The bare buffer makes the accessible output independent of the terminal.
func accessibleOutput(t *testing.T, consoleTheme *themeapi.Theme, testCase accessibleCase) string {
	t.Helper()
	var output bytes.Buffer
	prompter := prompt.New(prompt.Options{
		Theme:      consoleTheme,
		Input:      strings.NewReader(testCase.input),
		Output:     &output,
		Accessible: true,
	})
	if err := testCase.run(prompter); err != nil {
		t.Fatalf("%s: %v", testCase.name, err)
	}
	return output.String()
}

// TestPromptWithNoneProfileEmitsNoEscapes protects P0-5: ProfileNone must reach
// the prompt styles, not just the theme's own helpers.
func TestPromptWithNoneProfileEmitsNoEscapes(t *testing.T) {
	consoleTheme := themeapi.New(themeapi.WithProfile(themeapi.ProfileNone))

	for _, testCase := range accessibleCases() {
		t.Run(testCase.name, func(t *testing.T) {
			output := accessibleOutput(t, consoleTheme, testCase)
			// RED: Relatório P0-5 (C2 corrige). Today the title leaks
			// "\x1b[1;38;2;34;211;238m" and the answer labels leak 24-bit.
			if strings.ContainsRune(output, '\x1b') {
				t.Errorf("RED: Relatório P0-5 (C2 corrige): prompt output under ProfileNone = %q, want no escape sequences", output)
			}
		})
	}

}

// TestPromptWithNoneProfileAndEmptyPaletteEmitsNoEscapes covers the DevTools
// WithPalette(Palette{}) workaround: even an empty palette must not leave style
// escapes behind under ProfileNone.
func TestPromptWithNoneProfileAndEmptyPaletteEmitsNoEscapes(t *testing.T) {
	consoleTheme := themeapi.New(
		themeapi.WithProfile(themeapi.ProfileNone),
		themeapi.WithPalette(themeapi.Palette{}),
	)

	for _, testCase := range accessibleCases() {
		t.Run(testCase.name, func(t *testing.T) {
			output := accessibleOutput(t, consoleTheme, testCase)
			// RED: Relatório P0-5 (C2 corrige). Today the empty palette still
			// leaks the bold attribute ("\x1b[1m") from the title style.
			if strings.ContainsRune(output, '\x1b') {
				t.Errorf("RED: Relatório P0-5 (C2 corrige): prompt output with an empty palette under ProfileNone = %q, want no escape sequences", output)
			}
		})
	}
}

// TestPromptDowngradesColorsToItsProfile protects P0-5 for the smaller color
// profiles: 16-color and 256-color terminals must never receive 24-bit colors.
func TestPromptDowngradesColorsToItsProfile(t *testing.T) {
	for _, profile := range []themeapi.Profile{themeapi.ProfileANSI, themeapi.ProfileANSI256} {
		t.Run(profile.String(), func(t *testing.T) {
			consoleTheme := themeapi.New(themeapi.WithProfile(profile))
			for _, testCase := range accessibleCases() {
				t.Run(testCase.name, func(t *testing.T) {
					output := accessibleOutput(t, consoleTheme, testCase)
					// The line-mode output follows the theme profile: no 24-bit
					// color on the smaller profiles. The parameters are matched
					// inside the SGR because lipgloss merges them with attributes
					// ("\x1b[1;38;2;...").
					for _, sequence := range []string{"38;2;", "48;2;"} {
						if strings.Contains(output, sequence) {
							t.Errorf("RED: Relatório P0-5 (C2 corrige): %s output leaked %q: %q", profile, sequence, output)
						}
					}
					// The profile still carries a color, so the prompt must not
					// collapse to unstyled text either (C2 keeps the slot color).
					if !strings.ContainsRune(output, '\x1b') {
						t.Errorf("prompt output under %s has no styling at all: %q", profile, output)
					}
				})
			}

		})
	}
}
