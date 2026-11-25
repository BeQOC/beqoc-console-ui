// Package theme defines BeQOC's semantic terminal vocabulary: color roles,
// status symbols, a light/dark palette and plain rendering that degrades
// cleanly on every color profile.
//
// The package is a presentation domain. It owns no global state, writes no
// files and contains no product logic. Consumers construct a [Theme] with
// [New], keep it, and pass it to the spinner and prompt domains.
//
// Without an explicit [WithSymbols], the status symbols follow the process
// locale: UTF-8 terminals get [DefaultSymbols] and non-UTF-8 locales get
// [ASCIISymbols].
package theme

import (
	"image/color"
	"io"
	"os"

	"charm.land/lipgloss/v2"
)

// Role is a semantic style role.
type Role int

const (
	// RolePrimary is the main brand color.
	RolePrimary Role = iota
	// RoleAccent marks interactive or secondary emphasis.
	RoleAccent
	// RoleHint renders supporting text.
	RoleHint
	// RoleStrong renders bold text without a color.
	RoleStrong
	// RoleSuccess renders successful outcomes.
	RoleSuccess
	// RoleError renders failures.
	RoleError
	// RoleWarn renders warnings.
	RoleWarn
	// RoleInfo renders informational output.
	RoleInfo
	// RoleNone renders plain text.
	RoleNone
)

// Styles holds one prebuilt style per semantic role.
type Styles struct {
	Primary lipgloss.Style
	Accent  lipgloss.Style
	Hint    lipgloss.Style
	Strong  lipgloss.Style
	Success lipgloss.Style
	Error   lipgloss.Style
	Warn    lipgloss.Style
	Info    lipgloss.Style
	None    lipgloss.Style
}

// Theme is an immutable set of styles, symbols and rendering metadata. It is
// safe for concurrent use.
type Theme struct {
	profile Profile
	dark    bool
	palette Palette
	symbols Symbols
	colors  roleColors
	styles  Styles
}

type roleColors struct {
	primary color.Color
	accent  color.Color
	hint    color.Color
	success color.Color
	error   color.Color
	warn    color.Color
	info    color.Color
}

type config struct {
	output        io.Writer
	profile       Profile
	dark          bool
	palette       Palette
	symbols       Symbols
	customSymbols bool
}

// Option configures a [Theme].
type Option func(*config)

// WithOutput sets the stream used to detect the profile when
// [ProfileAuto] is selected. It defaults to os.Stdout and does not cause the
// theme to write anything.
func WithOutput(w io.Writer) Option {
	return func(c *config) {
		if w != nil {
			c.output = w
		}
	}
}

// WithProfile fixes the color profile. An explicit profile takes precedence
// over environment detection, which makes rendering deterministic.
func WithProfile(p Profile) Option {
	return func(c *config) {
		c.profile = p
	}
}

// WithDark selects the dark background variant. Dark is the default.
func WithDark(dark bool) Option {
	return func(c *config) {
		c.dark = dark
	}
}

// WithPalette replaces the palette.
func WithPalette(p Palette) Option {
	return func(c *config) {
		c.palette = p
	}
}

// WithSymbols replaces the status symbols. An explicit choice also wins over the
// locale, which otherwise selects [ASCIISymbols] outside UTF-8.
func WithSymbols(s Symbols) Option {
	return func(c *config) {
		c.symbols = s
		c.customSymbols = true
	}
}

// New builds a theme. Without options it returns the dark BeQOC theme with the
// profile detected from os.Stdout and the status symbols chosen by the process
// locale, falling back to [ASCIISymbols] outside UTF-8 unless [WithSymbols] was
// used.
func New(opts ...Option) *Theme {
	cfg := config{
		output:  os.Stdout,
		profile: ProfileAuto,
		dark:    true,
		palette: DefaultPalette(),
		symbols: DefaultSymbols(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.output == nil {
		cfg.output = os.Stdout
	}
	if !cfg.customSymbols && !localeAllowsUTF8() {
		cfg.symbols = ASCIISymbols()
	}

	t := &Theme{
		profile: resolveProfile(cfg.profile, cfg.output),
		dark:    cfg.dark,
		palette: cfg.palette,
		symbols: cfg.symbols,
	}
	t.colors = resolveColors(cfg.palette, cfg.dark, t.profile)
	t.styles = buildStyles(t.colors)
	return t
}

func resolveColors(p Palette, dark bool, profile Profile) roleColors {
	switch profile {
	case ProfileANSI:
		// Sixteen-color terminals do not share one RGB palette, and generic
		// conversion collapses amber and red into the same slot. Map each role
		// to the basic slot it conventionally owns so warnings stay
		// distinguishable from errors.
		return roleColors{
			primary: lipgloss.Color(ansiSlot(RolePrimary, dark)),
			accent:  lipgloss.Color(ansiSlot(RoleAccent, dark)),
			hint:    lipgloss.Color(ansiSlot(RoleHint, dark)),
			success: lipgloss.Color(ansiSlot(RoleSuccess, dark)),
			error:   lipgloss.Color(ansiSlot(RoleError, dark)),
			warn:    lipgloss.Color(ansiSlot(RoleWarn, dark)),
			info:    lipgloss.Color(ansiSlot(RoleInfo, dark)),
		}
	case ProfileNone, ProfileASCII:
		// Neither profile can show color, so the styles stay uncolored. Giving
		// them a color they cannot render made lipgloss emit a color sequence
		// that the profile then stripped, which left an empty "\x1b[m" pair
		// around every colored role under ASCII. Attributes such as bold and
		// italic are unaffected because they are set on separate styles.
		return roleColors{}
	}

	lightDark := lipgloss.LightDark(dark)
	pick := func(c Color) color.Color {
		light, darkVariant := c.Light, c.Dark
		if light == "" {
			light = darkVariant
		}
		if darkVariant == "" {
			darkVariant = light
		}
		return lightDark(lipgloss.Color(light), lipgloss.Color(darkVariant))
	}

	return roleColors{
		primary: pick(p.Primary),
		accent:  pick(p.Accent),
		hint:    pick(p.Hint),
		success: pick(p.Success),
		error:   pick(p.Error),
		warn:    pick(p.Warn),
		info:    pick(p.Info),
	}
}

// ansiSlot returns the basic ANSI color index for a role.
func ansiSlot(role Role, dark bool) string {
	if dark {
		switch role {
		case RolePrimary:
			return "14" // bright cyan
		case RoleAccent:
			return "12" // bright blue
		case RoleHint:
			return "8" // gray
		case RoleSuccess:
			return "10" // bright green
		case RoleError:
			return "9" // bright red
		case RoleWarn:
			return "11" // bright yellow
		case RoleInfo:
			return "13" // bright magenta
		default:
			return "7"
		}
	}

	switch role {
	case RolePrimary:
		return "6" // cyan
	case RoleAccent:
		return "4" // blue
	case RoleHint:
		return "8" // gray
	case RoleSuccess:
		return "2" // green
	case RoleError:
		return "1" // red
	case RoleWarn:
		return "3" // yellow
	case RoleInfo:
		return "5" // magenta
	default:
		return "7"
	}
}

func buildStyles(c roleColors) Styles {
	base := lipgloss.NewStyle()
	return Styles{
		Primary: base.Foreground(c.primary),
		Accent:  base.Foreground(c.accent),
		Hint:    base.Foreground(c.hint),
		Strong:  base.Bold(true),
		Success: base.Foreground(c.success),
		Error:   base.Foreground(c.error),
		Warn:    base.Foreground(c.warn),
		Info:    base.Foreground(c.info),
		None:    base,
	}
}

// Profile reports the color profile the theme renders for.
func (t *Theme) Profile() Profile {
	return t.profile
}

// Dark reports whether the dark palette variant is active.
func (t *Theme) Dark() bool {
	return t.dark
}

// Palette returns the theme palette.
func (t *Theme) Palette() Palette {
	return t.palette
}

// Symbols returns the configured status symbols.
func (t *Theme) Symbols() Symbols {
	return t.symbols
}

// Styles returns all prebuilt role styles.
func (t *Theme) Styles() Styles {
	return t.styles
}

// PrimaryStyle returns the primary style.
func (t *Theme) PrimaryStyle() lipgloss.Style { return t.styles.Primary }

// AccentStyle returns the accent style.
func (t *Theme) AccentStyle() lipgloss.Style { return t.styles.Accent }

// HintStyle returns the hint style.
func (t *Theme) HintStyle() lipgloss.Style { return t.styles.Hint }

// StrongStyle returns the bold style.
func (t *Theme) StrongStyle() lipgloss.Style { return t.styles.Strong }

// SuccessStyle returns the success style.
func (t *Theme) SuccessStyle() lipgloss.Style { return t.styles.Success }

// ErrorStyle returns the error style.
func (t *Theme) ErrorStyle() lipgloss.Style { return t.styles.Error }

// WarnStyle returns the warning style.
func (t *Theme) WarnStyle() lipgloss.Style { return t.styles.Warn }

// InfoStyle returns the info style.
func (t *Theme) InfoStyle() lipgloss.Style { return t.styles.Info }

// NoneStyle returns the plain style.
func (t *Theme) NoneStyle() lipgloss.Style { return t.styles.None }

// Style returns the style for role. Unknown roles render plain.
func (t *Theme) Style(role Role) lipgloss.Style {
	switch role {
	case RolePrimary:
		return t.styles.Primary
	case RoleAccent:
		return t.styles.Accent
	case RoleHint:
		return t.styles.Hint
	case RoleStrong:
		return t.styles.Strong
	case RoleSuccess:
		return t.styles.Success
	case RoleError:
		return t.styles.Error
	case RoleWarn:
		return t.styles.Warn
	case RoleInfo:
		return t.styles.Info
	default:
		return t.styles.None
	}
}

// Render applies style and downgrades the result to the theme profile. Use it
// for every string that reaches a terminal so non-terminal output stays plain.
func (t *Theme) Render(style lipgloss.Style, text ...string) string {
	return t.render(style, text...)
}

// Title renders primary, bold text.
func (t *Theme) Title(text string) string {
	return t.render(t.styles.Primary.Bold(true), text)
}

// Italic renders italic text without a color.
func (t *Theme) Italic(text string) string {
	return t.render(t.styles.None.Italic(true), text)
}

// Success renders a success line: symbol and message in the success color.
func (t *Theme) Success(message string) string {
	return t.render(t.styles.Success, t.symbols.Success+" "+message)
}

// SuccessHint renders a success symbol with a muted message.
func (t *Theme) SuccessHint(message string) string {
	return t.statusHint(t.styles.Success, t.symbols.Success, message)
}

// SuccessNone renders a success symbol with an uncolored message.
func (t *Theme) SuccessNone(message string) string {
	return t.statusNone(t.styles.Success, t.symbols.Success, message)
}

// Error renders an error line: symbol and message in the error color.
func (t *Theme) Error(message string) string {
	return t.render(t.styles.Error, t.symbols.Error+" "+message)
}

// ErrorHint renders an error symbol with a muted message.
func (t *Theme) ErrorHint(message string) string {
	return t.statusHint(t.styles.Error, t.symbols.Error, message)
}

// ErrorNone renders an error symbol with an uncolored message.
func (t *Theme) ErrorNone(message string) string {
	return t.statusNone(t.styles.Error, t.symbols.Error, message)
}

// Warn renders a warning line: symbol and message in the warning color.
func (t *Theme) Warn(message string) string {
	return t.render(t.styles.Warn, t.symbols.Warn+" "+message)
}

// WarnHint renders a warning symbol with a muted message.
func (t *Theme) WarnHint(message string) string {
	return t.statusHint(t.styles.Warn, t.symbols.Warn, message)
}

// WarnNone renders a warning symbol with an uncolored message.
func (t *Theme) WarnNone(message string) string {
	return t.statusNone(t.styles.Warn, t.symbols.Warn, message)
}

// Info renders an info line: symbol and message in the info color.
func (t *Theme) Info(message string) string {
	return t.render(t.styles.Info, t.symbols.Info+" "+message)
}

// InfoHint renders an info symbol with a muted message.
func (t *Theme) InfoHint(message string) string {
	return t.statusHint(t.styles.Info, t.symbols.Info, message)
}

// InfoNone renders an info symbol with an uncolored message.
func (t *Theme) InfoNone(message string) string {
	return t.statusNone(t.styles.Info, t.symbols.Info, message)
}

func (t *Theme) statusHint(style lipgloss.Style, symbol, message string) string {
	return t.render(style, symbol) + " " + t.render(t.styles.Hint, message)
}

func (t *Theme) statusNone(style lipgloss.Style, symbol, message string) string {
	return t.render(style, symbol) + " " + message
}
