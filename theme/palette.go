package theme

// Color is a semantic color with a light and a dark variant. Both variants are
// hex strings. When one variant is empty the other one is used, and when both
// are empty the role is rendered without color.
//
// Variants are truecolor values; the theme downgrades them for terminals with
// a smaller color profile.
type Color struct {
	Light string
	Dark  string
}

// Palette holds one [Color] per colored semantic role. Strong text and plain
// text carry no color of their own.
type Palette struct {
	Primary Color
	Accent  Color
	Hint    Color
	Success Color
	Error   Color
	Warn    Color
	Info    Color
}

// DefaultPalette returns the BeQOC palette: cyan primary, indigo accent, muted
// gray hints and conventional status colors. Every role has a distinct light
// and dark variant so the output stays readable on either background, and every
// variant keeps a contrast ratio of at least 4.5:1 against the backgrounds the
// console uses (#FFFFFF and #F3F4F6 for light, #000000, #0F172A, #1E1E1E and
// #282A36 for dark).
func DefaultPalette() Palette {
	return Palette{
		Primary: Color{Light: "#0E7490", Dark: "#22D3EE"},
		Accent:  Color{Light: "#4338CA", Dark: "#60A5FA"},
		Hint:    Color{Light: "#4B5563", Dark: "#9CA3AF"},
		Success: Color{Light: "#15803D", Dark: "#4ADE80"},
		Error:   Color{Light: "#B91C1C", Dark: "#F87171"},
		Warn:    Color{Light: "#854D0E", Dark: "#FBBF24"},
		Info:    Color{Light: "#1D4ED8", Dark: "#93C5FD"},
	}
}
