package prompt

import (
	"image/color"
	"math"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/BeQOC/beqoc-console-ui/theme"
)

// colorProfile mirrors the theme's internal capability mapping, so a style can
// be built with colors the target profile can actually render.
func colorProfile(profile theme.Profile) colorprofile.Profile {
	switch profile {
	case theme.ProfileASCII:
		return colorprofile.ASCII
	case theme.ProfileANSI:
		return colorprofile.ANSI
	case theme.ProfileANSI256:
		return colorprofile.ANSI256
	case theme.ProfileTrueColor:
		return colorprofile.TrueColor
	default:
		return colorprofile.NoTTY
	}
}

// huhTheme adapts a consoleui theme to huh. The consoleui theme already
// resolved its light or dark variant, so huh's own background flag is ignored
// on purpose: prompts always agree with the surrounding output.
//
// Every style honors the theme profile: ProfileNone carries no escape sequence
// at all, ProfileASCII keeps attributes but no color, and the smaller color
// profiles receive colors converted to their own palette instead of 24-bit
// values.
func huhTheme(consoleTheme *theme.Theme) huh.Theme {
	if consoleTheme == nil {
		consoleTheme = theme.New()
	}
	profile := colorProfile(consoleTheme.Profile())
	decorated := consoleTheme.Profile() != theme.ProfileNone

	return huh.ThemeFunc(func(bool) *huh.Styles {
		styles := consoleTheme.Styles()

		convert := func(value color.Color) color.Color {
			if value == nil {
				return nil
			}
			return profile.Convert(value)
		}
		bold := func(style lipgloss.Style) lipgloss.Style {
			if !decorated {
				return style
			}
			return style.Bold(true)
		}

		primary := convert(styles.Primary.GetForeground())
		accent := convert(styles.Accent.GetForeground())
		hint := convert(styles.Hint.GetForeground())
		success := convert(styles.Success.GetForeground())
		errorColor := convert(styles.Error.GetForeground())

		s := huh.ThemeBase(false)

		s.Focused.Title = bold(s.Focused.Title.Foreground(primary))
		s.Focused.NoteTitle = bold(s.Focused.NoteTitle.Foreground(primary))
		s.Focused.Directory = s.Focused.Directory.Foreground(accent)
		s.Focused.Description = s.Focused.Description.Foreground(hint)
		s.Focused.ErrorIndicator = s.Focused.ErrorIndicator.Foreground(errorColor)
		s.Focused.ErrorMessage = s.Focused.ErrorMessage.Foreground(errorColor)
		s.Focused.SelectSelector = s.Focused.SelectSelector.Foreground(accent)
		s.Focused.NextIndicator = s.Focused.NextIndicator.Foreground(accent)
		s.Focused.PrevIndicator = s.Focused.PrevIndicator.Foreground(accent)
		s.Focused.MultiSelectSelector = s.Focused.MultiSelectSelector.Foreground(accent)
		s.Focused.SelectedOption = s.Focused.SelectedOption.Foreground(success)
		s.Focused.SelectedPrefix = s.Focused.SelectedPrefix.Foreground(success)
		s.Focused.SelectedPrefix = s.Focused.SelectedPrefix.SetString(consoleTheme.Symbols().Success + " ")
		s.Focused.UnselectedPrefix = s.Focused.UnselectedPrefix.Foreground(hint)
		s.Focused.UnselectedOption = s.Focused.UnselectedOption.Foreground(hint)

		// The answer button keeps its text readable on the primary background:
		// black on the bright cyan of the dark theme (11.62:1), white on the
		// darker teal of the light theme (5.36:1).
		button := s.Focused.FocusedButton
		if primary != nil {
			button = bold(button.Foreground(convert(readableOn(primary))).Background(primary))
		} else {
			button = button.UnsetForeground().UnsetBackground().UnsetBold()
		}
		s.Focused.FocusedButton = button
		s.Focused.Next = button

		blurredButton := s.Focused.BlurredButton.UnsetForeground().UnsetBackground().UnsetBold()
		if hint != nil {
			blurredButton = blurredButton.Foreground(hint)
		}
		s.Focused.BlurredButton = blurredButton

		s.Focused.TextInput.Cursor = s.Focused.TextInput.Cursor.Foreground(primary)
		s.Focused.TextInput.CursorText = s.Focused.TextInput.CursorText.UnsetForeground()
		s.Focused.TextInput.Placeholder = s.Focused.TextInput.Placeholder.Foreground(hint)
		s.Focused.TextInput.Prompt = s.Focused.TextInput.Prompt.Foreground(accent)

		s.Help.ShortKey = bold(s.Help.ShortKey.Foreground(primary))
		s.Help.ShortDesc = s.Help.ShortDesc.Foreground(hint)
		s.Help.ShortSeparator = s.Help.ShortSeparator.Foreground(hint)
		s.Help.FullKey = bold(s.Help.FullKey.Foreground(primary))
		s.Help.FullDesc = s.Help.FullDesc.Foreground(hint)
		s.Help.FullSeparator = s.Help.FullSeparator.Foreground(hint)
		s.Help.Ellipsis = s.Help.Ellipsis.Foreground(hint)

		s.Blurred = s.Focused
		s.Blurred.Base = s.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
		s.Blurred.Card = s.Blurred.Base
		s.Blurred.NextIndicator = lipgloss.NewStyle()
		s.Blurred.PrevIndicator = lipgloss.NewStyle()

		s.Group.Title = s.Focused.Title
		s.Group.Description = s.Focused.Description
		return s
	})
}

// readableOn returns black or white, whichever contrasts more with background.
func readableOn(background color.Color) color.Color {
	black := lipgloss.Color("#000000")
	white := lipgloss.Color("#FFFFFF")
	if contrastRatio(black, background) >= contrastRatio(white, background) {
		return black
	}
	return white
}

// contrastRatio is the WCAG 2.1 contrast ratio between two colors.
func contrastRatio(first, second color.Color) float64 {
	high, low := relativeLuminance(first), relativeLuminance(second)
	if high < low {
		high, low = low, high
	}
	return (high + 0.05) / (low + 0.05)
}

func relativeLuminance(value color.Color) float64 {
	r, g, b, _ := value.RGBA()
	channel := func(component uint32) float64 {
		normalized := float64(component) / 65535
		if normalized <= 0.03928 {
			return normalized / 12.92
		}
		return math.Pow((normalized+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}
