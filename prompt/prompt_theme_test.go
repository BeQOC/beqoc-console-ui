package prompt_test

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/BeQOC/beqoc-console-ui/prompt"
	themeapi "github.com/BeQOC/beqoc-console-ui/theme"
)

// minimumContrast is the WCAG 2.1 AA ratio for normal-size text.
const minimumContrast = 4.5

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

// focusedButtonRender renders the answer button through the prompt theme. The
// button only exists in interactive mode, so its style is rendered directly:
// the accessible renderer never shows it, but the 24-bit leak lives there.
func focusedButtonRender(consoleTheme *themeapi.Theme) string {
	return prompt.HuhTheme(consoleTheme).Theme(false).Focused.FocusedButton.Render("Yes")
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

	t.Run("answer button", func(t *testing.T) {
		// RED: Relatório P0-5 (C2 corrige). Today the button keeps the hardcoded
		// white foreground and the primary background, so it emits 24-bit.
		if rendered := focusedButtonRender(consoleTheme); strings.ContainsRune(rendered, '\x1b') {
			t.Errorf("RED: Relatório P0-5 (C2 corrige): answer button under ProfileNone = %q, want no escape sequences", rendered)
		}
	})
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
					// RED: Relatório P0-5 (C2 corrige). Today HuhTheme keeps the
					// 24-bit foreground regardless of the theme profile. The
					// parameters are matched inside the SGR because lipgloss
					// merges them with attributes ("\x1b[1;38;2;...").
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

			t.Run("answer button", func(t *testing.T) {
				// RED: Relatório P0-5 (C2 corrige). The button hardcodes
				// #FFFFFF, which is 24-bit even where the theme already uses
				// ANSI slots.
				rendered := focusedButtonRender(consoleTheme)
				for _, sequence := range []string{"38;2;", "48;2;"} {
					if strings.Contains(rendered, sequence) {
						t.Errorf("RED: Relatório P0-5 (C2 corrige): %s answer button leaked %q: %q", profile, sequence, rendered)
					}
				}
			})
		})
	}
}

// TestPromptFocusedButtonMeetsContrast protects the dark-theme button, whose
// white-on-cyan pairing is unreadable.
func TestPromptFocusedButtonMeetsContrast(t *testing.T) {
	consoleTheme := themeapi.New(
		themeapi.WithProfile(themeapi.ProfileTrueColor),
		themeapi.WithDark(true),
	)
	button := prompt.HuhTheme(consoleTheme).Theme(true).Focused.FocusedButton

	ratio, ok := contrastBetweenColors(button.GetForeground(), button.GetBackground())
	if !ok {
		t.Fatalf("the focused button has no foreground or background: %+v", button)
	}
	// RED: Relatório P1-2 (C2 corrige). Today it is #FFFFFF on #22D3EE, 1.81:1.
	if ratio < minimumContrast {
		t.Errorf("RED: Relatório P1-2 (C2 corrige): focused button %s on %s = %.2f:1, want at least %.1f:1",
			hexOf(button.GetForeground()), hexOf(button.GetBackground()), ratio, minimumContrast)
	}
}

// contrastBetweenColors is the WCAG 2.1 contrast ratio between two opaque
// colors, using the same formula as theme/theme_contrast_test.go.
func contrastBetweenColors(foreground, background color.Color) (float64, bool) {
	if foreground == nil || background == nil {
		return 0, false
	}
	fr, fg, fb, _ := foreground.RGBA()
	br, bg, bb, _ := background.RGBA()
	high, low := luminanceFromRGBA(fr, fg, fb), luminanceFromRGBA(br, bg, bb)
	if high < low {
		high, low = low, high
	}
	return (high + 0.05) / (low + 0.05), true
}

func luminanceFromRGBA(r, g, b uint32) float64 {
	channel := func(value uint32) float64 {
		component := float64(value) / 65535
		if component <= 0.03928 {
			return component / 12.92
		}
		return math.Pow((component+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

func hexOf(value color.Color) string {
	r, g, b, _ := value.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
}
