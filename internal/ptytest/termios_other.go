//go:build !windows && !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package ptytest

import (
	"errors"
	"os"
)

func readTermios(*os.File) (Termios, error) {
	return Termios{}, errors.New("ptytest: terminal state is not supported on this platform")
}
