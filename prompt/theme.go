package prompt

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	console "github.com/BeQOC/beqoc-console-ui"
)

// Theme adapts a console theme to huh. The console theme already resolved its
// light or dark variant, so huh's own background flag is ignored on purpose:
// prompts always agree with the surrounding console output.
func Theme(theme *console.Theme) huh.Theme {
	if theme == nil {
		theme = console.New()
	}
	return huh.ThemeFunc(func(bool) *huh.Styles {
		styles := theme.Styles()

		primary := styles.Primary.GetForeground()
		accent := styles.Accent.GetForeground()
		hint := styles.Hint.GetForeground()
		success := styles.Success.GetForeground()
		errorColor := styles.Error.GetForeground()

		s := huh.ThemeBase(false)

		s.Focused.Title = s.Focused.Title.Foreground(primary).Bold(true)
		s.Focused.NoteTitle = s.Focused.NoteTitle.Foreground(primary).Bold(true)
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
		s.Focused.SelectedPrefix = s.Focused.SelectedPrefix.SetString(theme.Symbols().Success + " ")
		s.Focused.UnselectedPrefix = s.Focused.UnselectedPrefix.Foreground(hint)
		s.Focused.UnselectedOption = s.Focused.UnselectedOption.Foreground(hint)
		s.Focused.FocusedButton = s.Focused.FocusedButton.
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primary).
			Bold(true)
		s.Focused.Next = s.Focused.FocusedButton
		s.Focused.BlurredButton = s.Focused.BlurredButton.
			Foreground(hint).
			Background(lipgloss.Color(""))

		s.Focused.TextInput.Cursor = s.Focused.TextInput.Cursor.Foreground(primary)
		s.Focused.TextInput.CursorText = s.Focused.TextInput.CursorText.Foreground(lipgloss.Color(""))
		s.Focused.TextInput.Placeholder = s.Focused.TextInput.Placeholder.Foreground(hint)
		s.Focused.TextInput.Prompt = s.Focused.TextInput.Prompt.Foreground(accent)

		s.Help.ShortKey = s.Help.ShortKey.Foreground(primary).Bold(true)
		s.Help.ShortDesc = s.Help.ShortDesc.Foreground(hint)
		s.Help.ShortSeparator = s.Help.ShortSeparator.Foreground(hint)
		s.Help.FullKey = s.Help.FullKey.Foreground(primary).Bold(true)
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
