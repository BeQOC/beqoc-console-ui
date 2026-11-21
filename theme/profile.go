package theme

import (
	"bytes"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// Profile is the color capability of an output stream.
//
// A profile only affects how styles are emitted. Rendering never panics on a
// profile that lacks a given capability; colors and attributes that cannot be
// represented are dropped or downgraded.
type Profile int

const (
	// ProfileAuto detects the capability of the output stream, honoring
	// NO_COLOR and non-terminal output.
	ProfileAuto Profile = iota
	// ProfileNone emits no escape sequences at all.
	ProfileNone
	// ProfileASCII emits text attributes but no color.
	ProfileASCII
	// ProfileANSI emits the 16 basic ANSI colors.
	ProfileANSI
	// ProfileANSI256 emits the 256-color ANSI palette.
	ProfileANSI256
	// ProfileTrueColor emits 24-bit colors.
	ProfileTrueColor
)

// String implements [fmt.Stringer].
func (p Profile) String() string {
	switch p {
	case ProfileAuto:
		return "auto"
	case ProfileNone:
		return "none"
	case ProfileASCII:
		return "ascii"
	case ProfileANSI:
		return "ansi"
	case ProfileANSI256:
		return "ansi256"
	case ProfileTrueColor:
		return "truecolor"
	default:
		return "unknown"
	}
}

func (p Profile) colorProfile() colorprofile.Profile {
	switch p {
	case ProfileASCII:
		return colorprofile.ASCII
	case ProfileANSI:
		return colorprofile.ANSI
	case ProfileANSI256:
		return colorprofile.ANSI256
	case ProfileTrueColor:
		return colorprofile.TrueColor
	case ProfileNone, ProfileAuto:
		return colorprofile.NoTTY
	default:
		return colorprofile.NoTTY
	}
}

func profileFromColorProfile(p colorprofile.Profile) Profile {
	switch {
	case p >= colorprofile.TrueColor:
		return ProfileTrueColor
	case p >= colorprofile.ANSI256:
		return ProfileANSI256
	case p >= colorprofile.ANSI:
		return ProfileANSI
	case p >= colorprofile.ASCII:
		return ProfileASCII
	default:
		return ProfileNone
	}
}

// resolveProfile turns ProfileAuto into a concrete profile for out. An
// explicitly requested profile always wins, which keeps tests and callers
// that need a fixed capability deterministic.
func resolveProfile(requested Profile, out io.Writer) Profile {
	if requested != ProfileAuto {
		return requested
	}
	if noColorRequested() {
		return ProfileNone
	}
	return profileFromColorProfile(colorprofile.Detect(out, os.Environ()))
}

func noColorRequested() bool {
	return strings.TrimSpace(os.Getenv("NO_COLOR")) != ""
}

// render applies style and downgrades the result to the theme's profile.
func (t *Theme) render(style lipgloss.Style, text ...string) string {
	rendered := style.Render(text...)
	if t.profile == ProfileTrueColor {
		return rendered
	}

	var buf bytes.Buffer
	writer := colorprofile.Writer{Forward: &buf, Profile: t.profile.colorProfile()}
	_, _ = writer.WriteString(rendered)
	return buf.String()
}
