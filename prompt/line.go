package prompt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// The line mode is the prompt engine for everything that is not a terminal: a
// pipe, a script, a CI job or an explicit Accessible prompter. It reads one
// answer per line and prints plain text styled through the theme, so a
// ProfileNone or NO_COLOR theme stays free of escape sequences.
//
// A single bufio.Reader is kept per Prompter, which is what makes several
// prompts read consecutive lines from the same stream.

// lineResult is one answer read from the input.
type lineResult struct {
	text string
	err  error
}

// readLine reads one answer. A pre-canceled context fails immediately; a
// cancellation that arrives while the read is blocked is reported without
// losing the answer, because the pending read is kept for the next call.
func (p *Prompter) readLine(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", cancelError(err)
	}

	result := p.pending
	if result != nil {
		p.pending = nil
	} else {
		if p.reader == nil {
			p.reader = bufio.NewReader(p.input)
		}
		result = p.readLineAsync()
	}

	select {
	case read := <-result:
		switch {
		case read.err == nil:
			return trimLine(read.text), nil
		case errors.Is(read.err, io.EOF) && read.text != "":
			// The answer arrived without a trailing newline; accept it.
			return trimLine(read.text), nil
		case errors.Is(read.err, io.EOF):
			return "", ErrEOF
		default:
			return "", fmt.Errorf("prompt: read answer: %w", read.err)
		}
	case <-ctx.Done():
		// Keep the read: its answer belongs to the next prompt, not to this
		// one. A reader that never yields keeps one goroutine alive until the
		// caller closes it.
		p.pending = result
		return "", cancelError(ctx.Err())
	}
}

// readLineAsync starts one read and delivers its result on a buffered channel,
// so the goroutine never blocks after the caller has moved on.
func (p *Prompter) readLineAsync() <-chan lineResult {
	result := make(chan lineResult, 1)
	go func() {
		text, err := p.reader.ReadString('\n')
		result <- lineResult{text: text, err: err}
	}()
	return result
}

func trimLine(text string) string {
	return strings.TrimRight(text, "\r\n")
}

// printTitle writes a heading line through the theme, so it follows the color
// profile and stays escape-free where the profile asks for it.
func (p *Prompter) printTitle(text string) {
	fmt.Fprintln(p.output, p.theme.Render(p.theme.PrimaryStyle().Bold(true), text))
}

// printHint writes supporting text.
func (p *Prompter) printHint(text string) {
	fmt.Fprintln(p.output, p.theme.Render(p.theme.HintStyle(), text))
}

// printText writes a plain line.
func (p *Prompter) printText(text string) {
	fmt.Fprintln(p.output, p.theme.Render(p.theme.NoneStyle(), text))
}

// printError writes a rejected answer, in the theme's error role.
func (p *Prompter) printError(text string) {
	fmt.Fprintln(p.output, p.theme.Error(text))
}

// confirmLine asks a yes/no question until the answer is recognizable.
func (p *Prompter) confirmLine(ctx context.Context, opts ConfirmOptions) (bool, error) {
	title := opts.Title
	if title == "" {
		title = "Choose"
	}
	indicator := "[y/N]"
	if opts.Default {
		indicator = "[Y/n]"
	}

	first := true
	for {
		p.printTitle(title + " " + indicator)
		if first && opts.Description != "" {
			p.printHint(opts.Description)
		}
		first = false

		answer, err := p.readLine(ctx)
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		case "":
			return opts.Default, nil
		default:
			p.printError(fmt.Sprintf("%q is not an answer: type y or n", answer))
		}
	}
}

// inputLine asks for one line until it validates.
func (p *Prompter) inputLine(ctx context.Context, opts InputOptions) (string, error) {
	title := opts.Title
	if title == "" {
		title = "Input:"
	}

	first := true
	for {
		p.printTitle(title)
		if first && opts.Description != "" {
			p.printHint(opts.Description)
		}
		first = false

		answer, err := p.readLine(ctx)
		if err != nil {
			return "", err
		}
		value := strings.TrimSpace(answer)
		if value == "" {
			value = opts.Default
		}
		if opts.Validate != nil {
			if validationErr := opts.Validate(value); validationErr != nil {
				p.printError(validationErr.Error())
				continue
			}
		}
		return value, nil
	}
}

// selectLine prints the numbered choices and reads an index. It is a function
// because Go does not allow type parameters on methods.
func selectLine[T comparable](p *Prompter, ctx context.Context, opts SelectOptions[T]) (T, error) {
	var zero T

	title := opts.Title
	if title == "" {
		title = "Select:"
	}
	p.printTitle(title)
	if opts.Description != "" {
		p.printHint(opts.Description)
	}
	for index, choice := range opts.Choices {
		p.printText(fmt.Sprintf("%d. %s", index+1, choice.Label))
	}

	question := fmt.Sprintf("Enter a number between %d and %d: ", 1, len(opts.Choices))
	if len(opts.Choices) == 1 {
		question = "There is only one option available; enter the number 1: "
	}

	for {
		p.printHint(question)
		answer, err := p.readLine(ctx)
		if err != nil {
			return zero, err
		}
		trimmed := strings.TrimSpace(answer)
		if trimmed == "" && opts.Default != nil {
			return *opts.Default, nil
		}
		number, convertErr := strconv.Atoi(trimmed)
		if convertErr != nil || number < 1 || number > len(opts.Choices) {
			p.printError(fmt.Sprintf("%q is not a number between 1 and %d", answer, len(opts.Choices)))
			continue
		}
		return opts.Choices[number-1].Value, nil
	}
}
