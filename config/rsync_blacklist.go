package config

import (
	"strings"

	"github.com/tidwall/match"
)

// blacklistArgs contains a list of "forbidden" arguments to rsync. These
// are partially used by sshMaster.rsync or alter the behaviour of rsync
// in unwanted ways.
//
// This list is not exhaustive.
var blacklistArgs = []blacklistArg{
	{long: `--debug`, opt: true},               // generates too much noise
	{long: `--info`, opt: true},                // we already set -v, which influences both --debug and --info
	{long: `--verbose`, short: `-v`},           // should not be set multiple time (noise)
	{long: `--delete*`},                        // we're enforcing --delete --delete-excluded --delete-delay
	{long: `--del`},                            // shorthand for some other --delete-* flags
	{long: `--rsh`, short: `-e`, opt: true},    // is constructed separately
	{long: `--quiet`, short: `-q`},             // we actually want some output
	{long: `--force`},                          // irrelevant when --delete is set
	{long: `--include`, opt: true},             // is constructed separately
	{long: `--exclude`, opt: true},             // is constructed separately
	{long: `--filter`, short: `-f`, opt: true}, // overrides --include/--exclude
	{long: `--itemize-changes`, short: `-i`},   // defines a machine readable output
	{long: `--out-format`, opt: true},          // would override --itemize-changes
	{long: `--partial`},                        // "keep partially transferred files". nope.
	{long: `--progress`, short: "-P"},          // produces ANSI escape sequences for a human-readable progress meter
	{long: `--daemon`},                         // VERY bad idea to deamonize the rsync instance
}

// blacklistArg models an rsync argument, which can exist in long and
// short form  (--long vs. -l). If an argument swallow the next token,
// opt is true.
//
// long can actually be a simple match pattern.
type blacklistArg struct {
	long, short string
	opt         bool
}

func (bla *blacklistArg) Matches(arg string) (bool, int) {
	if bla.short != "" {
		if arg == bla.short {
			return true, bla.n()
		}
		if bla.opt && strings.HasPrefix(arg, bla.short) {
			// gotcha: -s42 does not swallow the next token
			return true, 0
		}
	}
	if bla.long != "" {
		if arg == bla.long {
			return true, bla.n()
		}
		if strings.ContainsRune(bla.long, '*') {
			// assume the user knows what he's doing
			return match.Match(arg, bla.long), bla.n()
		}
		if bla.opt {
			// gotcha: --long=arg does not swallow the next token
			return match.Match(arg, bla.long+"=*"), 0
		}
	}
	return false, 0
}

func (bla *blacklistArg) n() int {
	if bla.opt {
		return 1
	}
	return 0
}

// args removes blacklisted values from r.Arguments, to prevent you
// from shooting yourself in the foot.
func (r *RsyncConfig) args() []string {
	f := make([]string, 0, len(r.Arguments))

	for i := 0; i < len(r.Arguments); i++ {
		blacklisted := false
		arg := r.Arguments[i]

		for _, flag := range blacklistArgs {
			if matches, n := flag.Matches(arg); matches {
				i += n
				blacklisted = true
				break
			}
		}
		if !blacklisted {
			f = append(f, arg)
		}
	}
	return f
}
