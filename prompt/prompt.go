// Package prompt asks confirmation, selection and single-line input questions
// with the BeQOC console theme and returns plain values.
//
// A real terminal uses the huh form library. Pipes, scripts, CI jobs and an
// explicit Accessible prompter use a built-in line reader instead, which reads
// one answer per line, honors the caller context and never turns an input end
// into a silent answer.
//
// Only the common forms are exposed. Callers own the interaction policy; this
// package never retries a declined confirmation or inspects product state.
package prompt

import (
	"bufio"
	"context"
	"io"
	"os"
	"sync"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"

	"github.com/BeQOC/beqoc-console-ui/theme"
)

// ConfirmOptions describes a yes/no question.
type ConfirmOptions struct {
	Title       string
	Description string
	Affirmative string
	Negative    string
	Default     bool
}

// InputOptions describes a single-line text question.
type InputOptions struct {
	Title       string
	Description string
	Placeholder string
	Default     string
	Validate    func(string) error
}

// Choice is one selectable option. Label is shown to the user; Value is
// returned unchanged.
type Choice[T comparable] struct {
	Label string
	Value T
}

// SelectOptions describes a list question.
type SelectOptions[T comparable] struct {
	Title       string
	Description string
	Choices     []Choice[T]
	Default     *T
}

// Prompter asks questions with a fixed theme and input/output pair.
//
// A Prompter is meant to be used by one conversation at a time. Prompts are
// serialized by an internal mutex so the shared line reader and the rendered
// output never interleave, but callers should still ask one question at a time.
type Prompter struct {
	theme  *theme.Theme
	input  io.Reader
	output io.Writer
	// line selects the built-in reader: accessibility was requested, or a
	// stream is not a terminal.
	line bool

	mu      sync.Mutex
	reader  *bufio.Reader
	pending <-chan lineResult
}

// Options configure a [Prompter].
type Options struct {
	// Theme styles the forms. A default theme is used when nil.
	Theme *theme.Theme
	// Input supplies answers. Accessibility mode reads one line per answer;
	// interactive mode reads terminal keys.
	Input io.Reader
	// Output receives the rendered form.
	Output io.Writer
	// Accessible forces the line-oriented mode, which reads one answer per
	// line and needs no terminal. The line mode is also selected automatically
	// when Input or Output is not a terminal, so scripts and tests need not set
	// it.
	Accessible bool
}

// New builds a prompter.
func New(opts Options) *Prompter {
	consoleTheme := opts.Theme
	if consoleTheme == nil {
		consoleTheme = theme.New(theme.WithOutput(opts.Output))
	}
	input := opts.Input
	if input == nil {
		input = os.Stdin
	}
	output := opts.Output
	if output == nil {
		output = os.Stdout
	}
	return &Prompter{
		theme:  consoleTheme,
		input:  input,
		output: output,
		line:   opts.Accessible || !isTerminal(input) || !isTerminal(output),
	}
}

// Theme returns the prompter theme.
func (p *Prompter) Theme() *theme.Theme {
	return p.theme
}

// Confirm asks a yes/no question and returns the answer.
func (p *Prompter) Confirm(ctx context.Context, opts ConfirmOptions) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.line {
		return p.confirmLine(ctx, opts)
	}

	value := opts.Default
	field := huh.NewConfirm().
		Title(opts.Title).
		Value(&value)
	if opts.Description != "" {
		field.Description(opts.Description)
	}
	if opts.Affirmative != "" {
		field.Affirmative(opts.Affirmative)
	}
	if opts.Negative != "" {
		field.Negative(opts.Negative)
	}
	if err := p.runForm(ctx, field); err != nil {
		return false, err
	}
	return value, nil
}

// Input asks for a single line and returns the answer.
func (p *Prompter) Input(ctx context.Context, opts InputOptions) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.line {
		return p.inputLine(ctx, opts)
	}

	value := opts.Default
	field := huh.NewInput().
		Title(opts.Title).
		Value(&value)
	if opts.Description != "" {
		field.Description(opts.Description)
	}
	if opts.Placeholder != "" {
		field.Placeholder(opts.Placeholder)
	}
	if opts.Validate != nil {
		field.Validate(opts.Validate)
	}
	if err := p.runForm(ctx, field); err != nil {
		return "", err
	}
	return value, nil
}

// Select asks the user to pick one option. It is a function because Go does
// not allow type parameters on methods.
//
// An empty choice list returns [ErrNoChoices] and a default that is not one of
// the choices returns [ErrInvalidDefault], both before any input is read.
func Select[T comparable](ctx context.Context, p *Prompter, opts SelectOptions[T]) (T, error) {
	var zero T

	if len(opts.Choices) == 0 {
		return zero, ErrNoChoices
	}
	if opts.Default != nil && !hasChoice(opts.Choices, *opts.Default) {
		return zero, ErrInvalidDefault
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.line {
		return selectLine(p, ctx, opts)
	}

	options := make([]huh.Option[T], len(opts.Choices))
	for i, choice := range opts.Choices {
		options[i] = huh.NewOption(choice.Label, choice.Value)
	}

	value := zero
	if opts.Default != nil {
		value = *opts.Default
	}

	field := huh.NewSelect[T]().
		Title(opts.Title).
		Options(options...).
		Value(&value)
	if opts.Description != "" {
		field.Description(opts.Description)
	}
	if err := p.runForm(ctx, field); err != nil {
		return zero, err
	}
	return value, nil
}

// hasChoice reports whether value is one of the choices.
func hasChoice[T comparable](choices []Choice[T], value T) bool {
	for _, choice := range choices {
		if choice.Value == value {
			return true
		}
	}
	return false
}

// runForm runs the huh form on a real terminal.
func (p *Prompter) runForm(ctx context.Context, field huh.Field) error {
	form := huh.NewForm(huh.NewGroup(field)).
		WithTheme(HuhTheme(p.theme)).
		WithInput(p.input).
		WithOutput(p.output)
	return form.RunWithContext(ctx)
}

// isTerminal reports whether the stream is a real terminal.
func isTerminal(stream any) bool {
	file, ok := stream.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(file.Fd())
}
