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
- Status helpers for the `✔`, `✗`, `i` and `!` markers, plus `Hint` and `None`
  message variants for each status.
- One palette with a light and a dark value per role. Colors are picked with
  `lipgloss.LightDark` and degrade for truecolor, 256-color, 16-color and
  no-color output.
- A spinner that animates one line and replaces it with a final status line.
- A prompt wrapper for confirm, select and input questions, styled with the same
  theme and returning plain Go values.
- Deterministic rendering: pass `consoleui.WithProfile(...)` and the output is
  stable, with no terminal query and no environment guesswork.

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
	fmt.Println(theme.WarnHint("3 files skipped"))
	fmt.Println(theme.Render(theme.AccentStyle(), "next: run the quality gate"))
}
```

`theme.Success`, `theme.Error`, `theme.Warn` and `theme.Info` color the marker
and the message. The `Hint` variants keep the marker colored and mute the
message; the `None` variants leave the message uncolored:

```go
fmt.Println(theme.SuccessHint("wrote 3 files"))
fmt.Println(theme.ErrorNone("command failed"))
```

Symbols are configurable:

```go
theme := consoleui.New(consoleui.WithSymbols(consoleui.Symbols{
	Success: "OK",
	Error:   "NO",
	Warn:    "!",
	Info:    "?",
}))
```

Styles are available through `theme.Styles()`, `theme.Style(role)` and the
`PrimaryStyle`, `AccentStyle`, and so on accessors for composition with other
`lipgloss` code.

## Color profiles

`consoleui.New` detects the profile from `os.Stdout` by default. Detection honors
`NO_COLOR` and non-terminal output, which renders as plain text. To make output
deterministic, choose a profile explicitly:

```go
theme := consoleui.New(consoleui.WithProfile(consoleui.ProfileANSI256))
```

| Profile | Behavior |
| --- | --- |
| `ProfileAuto` | Detect from the output stream; `NO_COLOR` disables styling. |
| `ProfileNone` | No escape sequences at all. |
| `ProfileASCII` | Text attributes such as bold; no color. |
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
```

`Select` is a function because Go methods cannot declare type parameters.
Questions return plain values and errors; callers own retries and decisions.

Pass `Accessible: true` together with `Input` and `Output` to use huh's
line-oriented mode. It needs no terminal and is the supported way to drive
prompts from scripts and tests. `prompt.HuhTheme` exposes the same styling as a
regular `huh.Theme` for callers that compose their own forms.

## Project structure

| Path | Package | Purpose |
| --- | --- | --- |
| `consoleui.go`, `doc.go` | `consoleui` | Stable facade: `New`, options and theme re-exports. |
| `theme/` | `theme` | Roles, palette, profiles and symbols. Imports nothing internal. |
| `spinner/` | `spinner` | Print-style progress line over the theme domain. |
| `prompt/` | `prompt` | Themed huh forms over the theme domain. |

The domains are independently importable, for example
`github.com/BeQOC/beqoc-console-ui/theme`. The root facade imports all three
domains; the domains do not import the facade, so there is no cycle. The facade
re-exports the theme surface with type aliases, so a `*consoleui.Theme` is the
same type the spinner and prompt domains accept. It also names the `Spinner`
and `Prompter` types; their constructors and options stay in their domains.

## Dependencies

The dependency set is intentionally small and pinned in `go.mod`:

- `charm.land/lipgloss/v2` — styling and color rendering.
- `github.com/charmbracelet/colorprofile` — profile detection and downgrading.
- `charm.land/huh/v2` — prompt forms (brings Bubble Tea and Bubbles).
- `github.com/charmbracelet/x/term` — terminal detection for the spinner.

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
