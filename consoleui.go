package consoleui

import (
	"io"

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
	// Styles holds one prebuilt style per role.
	Styles = theme.Styles
	// Symbols are the markers that prefix status lines.
	Symbols = theme.Symbols
	// Theme is an immutable set of styles, symbols and rendering metadata.
	Theme = theme.Theme
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

// ParseProfile resolves a color profile name. Names are matched
// case-insensitively and the empty name means [ProfileAuto].
func ParseProfile(name string) (Profile, error) {
	return theme.ParseProfile(name)
}

// Profiles returns every color profile in display order.
func Profiles() []Profile {
	return theme.Profiles()
}

// ErrUnknownProfile reports a color profile name that [ParseProfile] does not
// know.
var ErrUnknownProfile = theme.ErrUnknownProfile
