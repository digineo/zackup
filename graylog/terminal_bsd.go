// +build darwin freebsd openbsd netbsd dragonfly

package graylog

import "golang.org/x/sys/unix"

const ioctlReadTermios = unix.TIOCGETA
