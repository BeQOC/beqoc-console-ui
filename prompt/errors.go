package prompt

import (
	"errors"
	"fmt"
	"io"
)

// ErrCanceled reports a canceled prompt: Ctrl-C, SIGINT or SIGTERM, or a
// canceled caller context. Cancellation that comes from the context also
// matches context.Canceled or context.DeadlineExceeded.
var ErrCanceled = errors.New("prompt: canceled")

// ErrEOF reports that the input ended before an answer was given. It wraps
// io.EOF, so errors.Is(err, io.EOF) is true.
var ErrEOF = fmt.Errorf("prompt: input closed: %w", io.EOF)

// ErrNoChoices reports a [Select] without choices.
var ErrNoChoices = errors.New("prompt: no choices")

// ErrInvalidDefault reports a [Select] default that is not one of its choices.
var ErrInvalidDefault = errors.New("prompt: default is not one of the choices")

// cancelError wraps a cancellation cause so callers can match both
// [ErrCanceled] and the context error that caused it.
func cancelError(cause error) error {
	return fmt.Errorf("%w: %w", ErrCanceled, cause)
}
