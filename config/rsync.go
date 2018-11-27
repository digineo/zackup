package config

import (
	"path/filepath"
	"strings"

	"github.com/tidwall/match"
)

// RsyncConfig holds config value for the rsync binary
type RsyncConfig struct {
	Included  []string `yaml:"include"`
	Excluded  []string `yaml:"exclude"`
	Arguments []string `yaml:"args"`

	// OverrideGlobalInclude (and other OverrideGlobal* fields) inhibits
	// the inheritance of global Included values (or other fields) when
	// set to true.
	OverrideGlobalInclude   bool `yaml:"override_global_include"`
	OverrideGlobalExclude   bool `yaml:"override_global_exclude"` // see OverrideGlobalInclude
	OverrideGlobalArguments bool `yaml:"override_global_args"`    // see OverrideGlobalInclude
}

// BuildArgVector creates an ARGV for rsync.
func (r *RsyncConfig) BuildArgVector(ssh, src, dst string) []string {
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	if !strings.HasSuffix(dst, "/") {
		dst += "/"
	}

	args := r.filter()               // whatever is configured for this host
	args = append(args, "-e", ssh)   // -e 'ssh -S controlPath -p port -x'
	args = append(args, r.args()...) // --include ... --exclude ...

	args = append(args,
		// delete from dest (also if excluded), but at the end
		"--delete", "--delete-excluded", "--delete-delay",
		// the following tunes logging capabilities
		"--itemize-changes", // TODO: make configurable
	)

	args = append(args, src, dst) // user@host:/ /zackup/host/
	return args
}

// filter builds the filter argument list (--include/--exclude) for rsync.
// This is modelled after BackupPC:
// https://github.com/backuppc/backuppc/blob/master/lib/BackupPC/Xfer/Rsync.pm#L234
//
// Most of the complexity is based in the fact that we do an rsync from
// `host:/`, i.e. we start to copy from the root directory of the remote
// host.
//
// TODO: could be simplified.
func (r *RsyncConfig) filter() (list []string) {
	// Original comments are marked as quote ("//>").
	//
	//> If the user wants to just include /home/craig, then we need to do create
	//> include/exclude pairs at each level:
	//>
	//>     --include /home --exclude /*
	//>     --include /home/craig --exclude /home/*
	//>
	//> It's more complex if the user wants to include multiple deep paths. For
	//> example, if they want /home/craig and /var/log, then we need this mouthfull:
	//>
	//>     --include /home --include /var --exclude /*
	//>     --include /home/craig --exclude /home/*
	//>     --include /var/log --exclude /var/*
	//>
	//> To make this easier we do all the includes first and all of the excludes at
	//> the end (hopefully they commute).

	var inc, exc []string
	incDone := make(map[string]struct{})
	excDone := make(map[string]struct{})

	for _, incl := range r.Included {
		file := filepath.Clean("/" + incl)
		if file == "/" {
			//> This is a special case: if the user specifies
			//> "/" then just include it and don't exclude "/*".
			if _, ok := incDone[file]; !ok {
				inc = append(inc, file)
			}
			continue
		}

		var f string
		elems := strings.Split(file[1:], "/")
		for _, elem := range elems {
			if elem == "" {
				//> preserve a tailing slash
				elem = "/"
			}

			fs := f + "/*"
			if _, ok := excDone[fs]; !ok {
				exc = append(exc, fs)
				excDone[fs] = struct{}{}
			}

			f += "/" + elem
			if _, ok := incDone[f]; !ok {
				inc = append(inc, f)
				incDone[f] = struct{}{}
			}
		}
	}

	for _, f := range inc {
		list = append(list, "--include="+f)
	}
	for _, f := range exc {
		list = append(list, "--exclude="+f)
	}
	for _, f := range r.Excluded {
		//> just append additional exclude lists onto the end
		list = append(list, "--exclude="+f)
	}
	return
}

// blacklistArgs contains a list of "forbidden" arguments to rsync. These
// are partially used by sshMaster.rsync or alter the behaviour of rsync
// in unwanted ways.
//
// This list is not exhaustive.
var blacklistArgs = []blacklistArg{
	{long: `--debug`, opt: true},
	{long: `--info`, opt: true},
	{long: `--verbose`, short: `-v`},
	{long: `--delete*`},
	{long: `--del`},
	{long: `--rsh`, short: `-e`, opt: true},
	{long: `--quiet`, short: `-q`},
	{long: `--force`},
	{long: `--include`, opt: true},
	{long: `--exclude`, opt: true},
	{long: `--filter`, short: `-f`, opt: true},
	{long: `--itemize-changes`, short: `-i`},
	{long: `--out-format`, opt: true},
	{long: `--partial`},
	{long: `--progress`, short: "-P"},
	{long: `--daemon`}, // VERY bad idea
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
	if bla.short != "" && arg == bla.short {
		return true, bla.n()
	}
	if bla.long != "" {
		if arg == bla.long {
			return true, bla.n()
		}
		if strings.IndexRune(bla.long, '*') > -1 {
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
