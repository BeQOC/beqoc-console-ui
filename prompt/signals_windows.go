//go:build windows

package prompt

import (
	"errors"
	"io"
	"os"
	"os/signal"
)

// notifySignals subscribes to the signal that cancels a prompt. Windows has no
// SIGTERM delivery like Unix, so only the interrupt is observed.
func notifySignals(signals chan<- os.Signal) {
	signal.Notify(signals, os.Interrupt)
}

// isTerminalGone reports whether a read error means the terminal hangup.
func isTerminalGone(err error) bool {
	return errors.Is(err, io.EOF)
}
