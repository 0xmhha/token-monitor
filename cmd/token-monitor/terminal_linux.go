//go:build linux

package main

import "golang.org/x/sys/unix"

// enableOutputProcessing re-enables output post-processing (OPOST) after
// term.MakeRaw() disables it. Without OPOST, \n is not converted to \r\n,
// causing a staircase effect in terminal output.
func enableOutputProcessing(fd int) {
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return
	}
	t.Oflag |= unix.OPOST
	unix.IoctlSetTermios(fd, unix.TCSETS, t) //nolint:errcheck,gosec // best-effort terminal restoration
}
