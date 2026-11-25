//go:build !windows

package prompt

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// notifySignals subscribes to the signals that cancel a prompt: the interrupt
// and the termination request a shell or a supervisor sends.
func notifySignals(signals chan<- os.Signal) {
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
}

// isTerminalGone reports whether a read error means the terminal hangup.
// Closing the master side makes the slave read fail with EIO; some readers
// report the same condition as an end of input.
func isTerminalGone(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, syscall.EIO)
}
