# beqoc-console-ui

Small, consistent terminal presentation for BeQOC command-line tools.

`beqoc-console-ui` is a thin wrapper over the Charm stack. It gives BeQOC tools
one visual vocabulary — semantic color roles, status symbols, light and dark
variants, a print-style spinner and themed prompts — without hiding the
underlying libraries.

## What it is

- A `consoleui.Theme` with prebuilt [`lipgloss`](https://charm.land/lipgloss) styles
  for the roles `primary`, `accent`, `hint`, `strong`, `success`, `error`,
  `warn`, `info` and `none`. The theme is its own domain; the root package is
  the facade existing callers already use.
- Status helpers for the `✔`, `✗`, `!` and `i` markers, with a symbol set that
  falls back to plain ASCII when the process locale is not UTF-8 (`LC_ALL`,
  `LC_CTYPE` or `LANG`; an unset locale counts as UTF-8).
- One palette with a light and a dark value per role. Colors are picked with
  `lipgloss.LightDark` and degrade for truecolor, 256-color, 16-color and
  no-color output.
- A spinner that animates one line and replaces it with a final status line.
- Prompts for confirm, select, multi-select and input questions, styled with the
  same theme, returning plain Go values and stable sentinel errors.
- Deterministic rendering: pass `consoleui.WithProfile(...)` and the output is
  stable, with no terminal query and no environment guesswork.
- Profile parsing (`ParseProfile`, `Profiles`) so configuration files and the
  command line share one vocabulary.

## What it is not

- Not a styling engine, layout system or component library. Use `lipgloss`
  directly when a screen needs more than these roles.
- Not a full-screen TUI framework. The spinner and prompts work from ordinary
  commands; there is no application loop to own.
- Not a source of product behavior. It never reads project state, writes files,
  retries decisions or prompts on its own.
- Not a global theme registry. There is no package-level mutable state:
  construct a theme, keep it, pass it where it is needed.

## Install

```sh
go get github.com/BeQOC/beqoc-console-ui
```

Requires Go 1.25 or newer.

The import path ends in `beqoc-console-ui`, while the package name is
`consoleui`, so imports need no alias:

```go
import "github.com/BeQOC/beqoc-console-ui"
```

## Theme and symbols

```go
package main

import (
	"fmt"

	"github.com/BeQOC/beqoc-console-ui"
)

func main() {
	theme := consoleui.New(consoleui.WithDark(true))

	fmt.Println(theme.Title("BeQOC"))
	fmt.Println(theme.Success("configuration valid"))
	fmt.Println(theme.Warn("3 files skipped"))
	fmt.Println(theme.Render(theme.AccentStyle(), "next: run the quality gate"))
}
```

`theme.Success`, `theme.Error`, `theme.Warn` and `theme.Info` color the marker
and the message. `theme.Title` and `theme.Italic` render attributes without a
color role.

Symbols are configurable:

```go
theme := consoleui.New(consoleui.WithSymbols(consoleui.Symbols{
	Success: "OK",
	Error:   "NO",
	Warn:    "!",
	Info:    "?",
}))
```

Styles are available through `theme.Styles()` and the `PrimaryStyle`,
`AccentStyle`, `HintStyle`, `StrongStyle`, `SuccessStyle`, `ErrorStyle`,
`WarnStyle`, `InfoStyle` and `NoneStyle` accessors for composition with other
`lipgloss` code. `theme.Render` applies a style and downgrades the result to the
theme profile, which is the supported way to emit styled text.

## Color profiles

`consoleui.New` detects the profile from `os.Stdout` by default. Detection honors
`NO_COLOR` and non-terminal output, which renders as plain text. To make output
deterministic, choose a profile explicitly:

```go
theme := consoleui.New(consoleui.WithProfile(consoleui.ProfileANSI256))
```

Names resolve through the same parser the DevTools configuration uses:

```go
profile, err := consoleui.ParseProfile("ansi256") // case-insensitive, "" means auto
if err != nil {
	// errors.Is(err, consoleui.ErrUnknownProfile)
}
for _, profile := range consoleui.Profiles() {
	fmt.Println(profile) // auto, none, ascii, ansi, ansi256, truecolor
}
```

| Profile | Behavior |
| --- | --- |
| `ProfileAuto` | Detect from the output stream; `NO_COLOR` renders as `ProfileNone`. |
| `ProfileNone` | No escape sequences at all. |
| `ProfileASCII` | Text attributes such as bold or italic; no color and no empty SGR. |
| `ProfileANSI` | The 16 basic colors, mapped semantically per role. |
| `ProfileANSI256` | The 256-color palette. |
| `ProfileTrueColor` | 24-bit colors. |

At 16 colors, roles map to distinct basic slots rather than being converted by
nearest RGB distance: cyan primary, blue accent, magenta info, gray hint, green
success, yellow warning and red error. This keeps warnings out of the error
color and info out of the accent color. The palette therefore controls the
truecolor and 256-color profiles; the 16-color profile ignores the palette and
uses these standard slots.

Light and dark variants are part of the palette:

```go
light := consoleui.New(consoleui.WithDark(false))
dark := consoleui.New(consoleui.WithDark(true))
```

Customize colors with `consoleui.WithPalette`. Each role has a `Light` and a
`Dark` hex value:

```go
palette := consoleui.DefaultPalette()
palette.Primary = consoleui.Color{Light: "#0E7490", Dark: "#22D3EE"}
theme := consoleui.New(consoleui.WithPalette(palette))
```

Rendering is fail-safe: an empty or invalid color renders without color on the
profiles that use the palette, and never panics. `ProfileANSI` ignores the
palette and always uses its semantic basic slots.

## Spinner

```go
sp := spinner.New("Resolving dependencies", spinner.WithTheme(theme))
sp.Start()
// work happens here
sp.Succeed("dependencies resolved")
```

`Succeed`, `Fail`, `Warn` and `Info` clear the animation line and print the
final status in its place. `Update` changes the text while work runs, and
`Stop` clears the line without printing a status.

```go
sp := spinner.New("Installing", spinner.WithTheme(theme))
sp.Start()
sp.Update("Installing packages")
sp.Succeed()
```

The spinner is implemented as a single `time.Ticker` goroutine that draws
frames with `lipgloss`, rather than a Bubble Tea program. A spinner needs one
animated line and an in-place final message, so one ticker is enough; it stays
usable from plain commands and does not take over standard input. When the
writer is not a terminal (or `TERM=dumb`), it does not animate and only prints
the final line.

Every frame is truncated to the terminal width read before the draw, so a
resize is followed on the next frame and a narrow terminal never receives a
wrapped line. `spinner.WithWidth` fixes the width for deterministic output, and
`spinner.ASCIIFrames` offers the plain `| / - \` set:

```go
sp := spinner.New("Working",
	spinner.WithTheme(theme),
	spinner.WithFrames(spinner.ASCIIFrames()),
	spinner.WithWidth(40),
)
```

A write error stops the animation and is available from `sp.Err()`. The spinner
never installs signal handlers: cancellation belongs to the caller. Pass a
context with `spinner.WithContext` and cancel it from the signal handler you
own; the spinner clears its line and stops its frame loop.

## Prompts

```go
p := prompt.New(prompt.Options{Theme: theme})

ok, err := p.Confirm(ctx, prompt.ConfirmOptions{
	Title: "Apply these changes?",
})
if err != nil {
	return err
}

name, err := p.Input(ctx, prompt.InputOptions{
	Title:       "Project name",
	Placeholder: "my-project",
})
if err != nil {
	return err
}

target, err := prompt.Select(ctx, p, prompt.SelectOptions[string]{
	Title: "Target",
	Choices: []prompt.Choice[string]{
		{Label: "Local", Value: "local"},
		{Label: "Remote", Value: "remote"},
	},
})

targets, err := prompt.MultiSelect(ctx, p, prompt.MultiSelectOptions[string]{
	Title: "Targets",
	Choices: []prompt.Choice[string]{
		{Label: "Local", Value: "local"},
		{Label: "Remote", Value: "remote"},
	},
	Defaults: []string{"local"},
})
```

`Select` and `MultiSelect` are functions because Go methods cannot declare type
parameters. Questions return plain values and errors; callers own retries and
decisions. Multi-select returns the selected values in the order of the choices
and never repeats one.

On a terminal the form library draws the prompts. When `Input` or `Output` is
not a terminal the package switches to its own line reader, so scripts and tests
need no extra flag; `Accessible: true` forces that mode anywhere. In line mode
`MultiSelect` understands a number or a value to toggle, `?text` to search and
`done` to finish.

Errors are stable sentinels: `prompt.ErrCanceled` (Ctrl-C, SIGINT, SIGTERM, a
closed terminal or a canceled context, which also matches
`context.Canceled`/`context.DeadlineExceeded`), `prompt.ErrEOF` (a closed input
or Ctrl-D; it wraps `io.EOF`), `prompt.ErrNoChoices` and
`prompt.ErrInvalidDefault`.

While a prompt is open on a real terminal it intercepts `SIGINT` and `SIGTERM`
(only `os.Interrupt` on Windows) and removes the handler when the prompt
returns. A signal, or Ctrl-C read as a key, ends the form with
`prompt.ErrCanceled`, so the library never terminates the process by itself:
what a signal means outside a prompt is the owner's decision. The line reader
used for pipes and scripts installs no handler.

## Project structure

| Path | Package | Purpose |
| --- | --- | --- |
| `consoleui.go`, `doc.go` | `consoleui` | Stable facade: `New`, options, profile parsing and theme re-exports. |
| `theme/` | `theme` | Palettes, profiles, styles and symbols. Imports nothing internal. |
| `spinner/` | `spinner` | Print-style progress line over the theme domain. |
| `prompt/` | `prompt` | Themed huh forms over the theme domain. |

The domains are independently importable, for example
`github.com/BeQOC/beqoc-console-ui/theme`. The root facade imports only the theme
domain, so a caller that imports it for the theme does not build the form
library or a terminal UI; a test enforces that boundary. The facade re-exports
the theme surface with type aliases, so a `*consoleui.Theme` is the same type
the spinner and prompt domains accept.

## Dependencies

The dependency set is intentionally small and pinned in `go.mod`:

- `charm.land/lipgloss/v2` — styling and color rendering (theme, root).
- `github.com/charmbracelet/colorprofile` — profile detection and downgrading.
- `github.com/charmbracelet/x/ansi` — visual-width truncation for the spinner.
- `github.com/charmbracelet/x/term` — terminal detection.
- `charm.land/huh/v2` — prompt forms (brings Bubble Tea and Bubbles), used only
  by the `prompt` domain.
- `github.com/creack/pty` — real pseudo-terminals in tests only.

Nothing is vendored.

## Development

```sh
go build ./...
go vet ./...
gofmt -l .
go test ./... -count=1
```

Golden files live in `theme/testdata/golden`. Refresh them after an intentional
rendering change:

```sh
go test ./theme -run TestThemeGoldenByProfile -update
```

## License

MIT. See [LICENSE](LICENSE).
