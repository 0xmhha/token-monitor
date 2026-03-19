//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package main

import "golang.org/x/sys/unix"

// enableOutputProcessing re-enables output post-processing (OPOST) after
// term.MakeRaw() disables it. Without OPOST, \n is not converted to \r\n,
// causing a staircase effect in terminal output.
func enableOutputProcessing(fd int) {
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return
	}
	t.Oflag |= unix.OPOST
	_ = unix.IoctlSetTermios(fd, unix.TIOCSETA, t)
}
