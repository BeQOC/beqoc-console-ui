package console

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

// DefaultPalette returns the BeQOC palette: cyan primary, blue accent, muted
// gray hints and conventional status colors. Every role has a distinct light
// and dark variant so the output stays readable on either background.
func DefaultPalette() Palette {
	return Palette{
		Primary: Color{Light: "#0E7490", Dark: "#22D3EE"},
		Accent:  Color{Light: "#1D4ED8", Dark: "#60A5FA"},
		Hint:    Color{Light: "#4B5563", Dark: "#9CA3AF"},
		Success: Color{Light: "#15803D", Dark: "#4ADE80"},
		Error:   Color{Light: "#B91C1C", Dark: "#F87171"},
		Warn:    Color{Light: "#A16207", Dark: "#FBBF24"},
		Info:    Color{Light: "#1D4ED8", Dark: "#93C5FD"},
	}
}
