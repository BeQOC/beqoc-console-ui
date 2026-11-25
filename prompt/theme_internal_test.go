package prompt

import (
	"fmt"
	"image/color"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/BeQOC/beqoc-console-ui/theme"
)

// The huh adapter is internal (the public API must not name huh), so the tests
// that inspect the adapted styles live inside the package.

// minimumContrast is the WCAG 2.1 AA ratio for normal-size text.
const minimumContrast = 4.5

func TestThemeUsesConsoleColors(t *testing.T) {
	consoleTheme := theme.New(theme.WithProfile(theme.ProfileTrueColor), theme.WithDark(true))

	styles := huhTheme(consoleTheme).Theme(false)
	if got, want := styles.Focused.Title.GetForeground(), consoleTheme.Styles().Primary.GetForeground(); !reflect.DeepEqual(got, want) {
		t.Errorf("title foreground = %v, want %v", got, want)
	}
	if got, want := styles.Focused.ErrorMessage.GetForeground(), consoleTheme.Styles().Error.GetForeground(); !reflect.DeepEqual(got, want) {
		t.Errorf("error foreground = %v, want %v", got, want)
	}
	if got, want := styles.Focused.Description.GetForeground(), consoleTheme.Styles().Hint.GetForeground(); !reflect.DeepEqual(got, want) {
		t.Errorf("description foreground = %v, want %v", got, want)
	}
}

func TestNilThemeFallsBackToDefault(t *testing.T) {
	fallback := huhTheme(nil).Theme(true)
	explicit := huhTheme(theme.New()).Theme(true)
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

	prompter := New(Options{})
	if prompter.Theme() == nil {
		t.Fatal("New used a nil theme")
	}
}

// TestFocusedButtonFollowsTheProfile protects the answer button: no escape under
// ProfileNone and no 24-bit color on the smaller profiles.
func TestFocusedButtonFollowsTheProfile(t *testing.T) {
	button := func(profile theme.Profile) string {
		consoleTheme := theme.New(theme.WithProfile(profile))
		return huhTheme(consoleTheme).Theme(false).Focused.FocusedButton.Render("Yes")
	}

	if got := button(theme.ProfileNone); strings.ContainsRune(got, '\x1b') {
		t.Errorf("answer button under ProfileNone = %q, want no escape sequences", got)
	}
	for _, profile := range []theme.Profile{theme.ProfileANSI, theme.ProfileANSI256} {
		got := button(profile)
		for _, sequence := range []string{"38;2;", "48;2;"} {
			if strings.Contains(got, sequence) {
				t.Errorf("%s answer button leaked %q: %q", profile, sequence, got)
			}
		}
	}
}

// TestFocusedButtonMeetsContrast protects the dark-theme button text, which must
// stay readable on the primary background.
func TestFocusedButtonMeetsContrast(t *testing.T) {
	consoleTheme := theme.New(
		theme.WithProfile(theme.ProfileTrueColor),
		theme.WithDark(true),
	)
	button := huhTheme(consoleTheme).Theme(true).Focused.FocusedButton

	ratio, ok := contrastBetweenColors(button.GetForeground(), button.GetBackground())
	if !ok {
		t.Fatalf("the focused button has no foreground or background: %+v", button)
	}
	if ratio < minimumContrast {
		t.Errorf("focused button %s on %s = %.2f:1, want at least %.1f:1",
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
