package theme

// Symbols are the markers that prefix status lines.
type Symbols struct {
	Success string
	Error   string
	Warn    string
	Info    string
}

// ASCIISymbols returns the status markers for terminals that cannot show the
// Unicode symbols: "+" for success, "x" for an error, and the "!" and "i"
// markers the default set already uses. Every byte is printable ASCII.
func ASCIISymbols() Symbols {
	return Symbols{
		Success: "+",
		Error:   "x",
		Warn:    "!",
		Info:    "i",
	}
}

// DefaultSymbols returns the BeQOC status markers.
func DefaultSymbols() Symbols {
	return Symbols{
		Success: "✔",
		Error:   "✗",
		Warn:    "!",
		Info:    "i",
	}
}
