package graylog

// This is the core of golang.org/x/crypto/ssh/terminal.IsTerminal,
// without the need to import the SSH package into every project.

import (
	"os"

	"golang.org/x/sys/unix"
)

func isTerminal() bool {
	fd := int(os.Stdout.Fd())
	_, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	return err == nil
}
