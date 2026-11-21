package consoleui

import (
	"io"

	"github.com/BeQOC/beqoc-console-ui/prompt"
	"github.com/BeQOC/beqoc-console-ui/spinner"
	"github.com/BeQOC/beqoc-console-ui/theme"
)

// Theme and its supporting values are re-exported from the theme domain so
// existing callers keep using one import path. These are type aliases, not
// wrappers: a *consoleui.Theme is identical to *theme.Theme and passes to the
// spinner and prompt domains without conversion.
type (
	// Color is a semantic color with light and dark variants.
	Color = theme.Color
	// Option configures a Theme.
	Option = theme.Option
	// Palette holds one color per colored role.
	Palette = theme.Palette
	// Profile is a terminal color capability.
	Profile = theme.Profile
	// Role is a semantic style role.
	Role = theme.Role
	// Styles holds one prebuilt style per role.
	Styles = theme.Styles
	// Symbols are the markers that prefix status lines.
	Symbols = theme.Symbols
	// Theme is an immutable set of styles, symbols and rendering metadata.
	Theme = theme.Theme

	// Spinner is the print-style spinner type. Construct it with spinner.New.
	Spinner = spinner.Spinner
	// Prompter is the prompt type. Construct it with prompt.New.
	Prompter = prompt.Prompter
)

const (
	// ProfileAuto detects the capability of the output stream.
	ProfileAuto = theme.ProfileAuto
	// ProfileNone emits no escape sequences.
	ProfileNone = theme.ProfileNone
	// ProfileASCII emits text attributes without color.
	ProfileASCII = theme.ProfileASCII
	// ProfileANSI emits the 16 basic ANSI colors.
	ProfileANSI = theme.ProfileANSI
	// ProfileANSI256 emits the 256-color palette.
	ProfileANSI256 = theme.ProfileANSI256
	// ProfileTrueColor emits 24-bit colors.
	ProfileTrueColor = theme.ProfileTrueColor
)

const (
	// RolePrimary is the main brand color.
	RolePrimary = theme.RolePrimary
	// RoleAccent marks interactive or secondary emphasis.
	RoleAccent = theme.RoleAccent
	// RoleHint renders supporting text.
	RoleHint = theme.RoleHint
	// RoleStrong renders bold text without a color.
	RoleStrong = theme.RoleStrong
	// RoleSuccess renders successful outcomes.
	RoleSuccess = theme.RoleSuccess
	// RoleError renders failures.
	RoleError = theme.RoleError
	// RoleWarn renders warnings.
	RoleWarn = theme.RoleWarn
	// RoleInfo renders informational output.
	RoleInfo = theme.RoleInfo
	// RoleNone renders plain text.
	RoleNone = theme.RoleNone
)

// New builds a theme. Without options it returns the dark BeQOC theme with the
// profile detected from os.Stdout.
func New(opts ...Option) *Theme {
	return theme.New(opts...)
}

// WithOutput sets the stream used to detect the profile for ProfileAuto.
func WithOutput(w io.Writer) Option {
	return theme.WithOutput(w)
}

// WithProfile fixes the color profile.
func WithProfile(p Profile) Option {
	return theme.WithProfile(p)
}

// WithDark selects the dark background variant.
func WithDark(dark bool) Option {
	return theme.WithDark(dark)
}

// WithPalette replaces the palette.
func WithPalette(p Palette) Option {
	return theme.WithPalette(p)
}

// WithSymbols replaces the status symbols.
func WithSymbols(s Symbols) Option {
	return theme.WithSymbols(s)
}

// DefaultPalette returns the BeQOC palette.
func DefaultPalette() Palette {
	return theme.DefaultPalette()
}

// DefaultSymbols returns the BeQOC status markers.
func DefaultSymbols() Symbols {
	return theme.DefaultSymbols()
}
