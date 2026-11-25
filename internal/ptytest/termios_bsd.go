//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package ptytest

import (
	"os"

	"golang.org/x/sys/unix"
)

func readTermios(file *os.File) (Termios, error) {
	return queryTermios(file, unix.TIOCGETA)
}

func queryTermios(file *os.File, request uint) (Termios, error) {
	state, err := unix.IoctlGetTermios(int(file.Fd()), request)
	if err != nil {
		return Termios{}, err
	}
	return Termios{
		Canonical: state.Lflag&unix.ICANON != 0,
		Echo:      state.Lflag&unix.ECHO != 0,
		Signals:   state.Lflag&unix.ISIG != 0,
	}, nil
}
