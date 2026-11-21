package consoleui

// Symbols are the markers that prefix status lines.
type Symbols struct {
	Success string
	Error   string
	Warn    string
	Info    string
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
