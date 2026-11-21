// Package prompt wraps the huh form library with the BeQOC console theme and
// plain return values.
//
// Only the common forms are exposed: confirmation, selection and single-line
// input. Callers own the interaction policy; this package never retries a
// declined confirmation or inspects product state.
package prompt

import (
	"context"
	"io"

	"charm.land/huh/v2"

	console "github.com/BeQOC/beqoc-console-ui"
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

// Prompter runs forms with a fixed theme and input/output pair.
type Prompter struct {
	theme      *console.Theme
	input      io.Reader
	output     io.Writer
	accessible bool
}

// Options configure a [Prompter].
type Options struct {
	// Theme styles the forms. A default theme is used when nil.
	Theme *console.Theme
	// Input supplies answers. Accessibility mode reads one line per answer;
	// interactive mode reads terminal keys.
	Input io.Reader
	// Output receives the rendered form.
	Output io.Writer
	// Accessible selects huh's line-oriented mode, which needs no terminal.
	// It is the supported way to drive prompts from tests or scripts.
	Accessible bool
}

// New builds a prompter.
func New(opts Options) *Prompter {
	theme := opts.Theme
	if theme == nil {
		theme = console.New()
	}
	return &Prompter{
		theme:      theme,
		input:      opts.Input,
		output:     opts.Output,
		accessible: opts.Accessible,
	}
}

// Theme returns the prompter theme.
func (p *Prompter) Theme() *console.Theme {
	return p.theme
}

// Confirm asks a yes/no question and returns the answer.
func (p *Prompter) Confirm(ctx context.Context, opts ConfirmOptions) (bool, error) {
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
	if err := p.run(ctx, field); err != nil {
		return false, err
	}
	return value, nil
}

// Input asks for a single line and returns the answer.
func (p *Prompter) Input(ctx context.Context, opts InputOptions) (string, error) {
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
	if err := p.run(ctx, field); err != nil {
		return "", err
	}
	return value, nil
}

// Select asks the user to pick one option. It is a function because Go does
// not allow type parameters on methods.
func Select[T comparable](ctx context.Context, p *Prompter, opts SelectOptions[T]) (T, error) {
	var zero T

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
	if err := p.run(ctx, field); err != nil {
		return zero, err
	}
	return value, nil
}

func (p *Prompter) run(ctx context.Context, field huh.Field) error {
	form := huh.NewForm(huh.NewGroup(field)).
		WithTheme(Theme(p.theme)).
		WithAccessible(p.accessible)
	if p.input != nil {
		form.WithInput(p.input)
	}
	if p.output != nil {
		form.WithOutput(p.output)
	}
	return form.RunWithContext(ctx)
}
