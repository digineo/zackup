package app

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
)

type sshMaster struct {
	host string
	user string
	port uint16

	mu          *sync.Mutex     // lock for start/stop
	wg          *sync.WaitGroup // for execute/rsync
	controlPath string          // SSH multiplexing socket
	tunnel      *osexec.Cmd     // foreground SSH process
}

func newSSHMaster(host string, port uint16, user string) *sshMaster {
	return &sshMaster{
		host: host,
		user: user,
		port: port,

		controlPath: filepath.Join(RootDataset, host, ".sshctrl"),

		mu: &sync.Mutex{},
		wg: &sync.WaitGroup{},
	}
}

// ssh user@host -p port -o ControlMaster=yes -o ControlPath=/zackup/ssh/$uuid
func (c *sshMaster) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tunnel != nil {
		return errors.New("already established")
	}

	cmd := osexec.Command("ssh",
		"-o", "ControlMaster=yes",
		"-S", c.controlPath, // == -oControlPath=...
		"-n", // disable stdin
		"-N", // do not execute command on remote server
		"-T", // disable TTY allocation
		"-x", // disable X11 forwarding
		"-p", strconv.Itoa(int(c.port)),
		"-l", c.user,
		c.host,
	)

	if err := cmd.Start(); err != nil {
		return err
	}
	c.tunnel = cmd
	return nil
}

func (c *sshMaster) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tunnel == nil {
		return
	}

	l := log.WithFields(logrus.Fields{
		"prefix": "ssh.master",
		"host":   c.host,
	})

	// wait for running commands to finish
	c.wg.Wait()

	if err := c.tunnel.Process.Signal(syscall.SIGINT); err != nil {
		l.WithError(err).Warn("closing with interrupt failed, send kill")
		if err = c.tunnel.Process.Kill(); err != nil {
			l.WithError(err).Error("closing with kill failed")
		}
	}

	if err := c.tunnel.Wait(); err != nil {
		l.WithError(err).Warn("unexpected termination")
	}

	c.tunnel = nil
}

// echo script | ssh -oControlPath=... host /bin/sh -esx
func (c *sshMaster) execute(script []string) error {
	c.wg.Add(1)
	defer c.wg.Done()

	cmd := osexec.Command("ssh",
		"-S", c.controlPath, // == -oControlPath=...
		"-p", strconv.Itoa(int(c.port)),
		"-l", c.user,
		"-x", // disable X11 forwarding
		c.host,
		"/bin/sh", "-esx",
	)

	var o, e bytes.Buffer
	cmd.Stdout = &o
	cmd.Stderr = &e

	logerr := func(err error, msg string, extra ...string) {
		f := logrus.Fields{
			logrus.ErrorKey: err,
			"prefix":        "ssh.execute",
			"host":          c.host,
			"stdout":        o.String(),
			"stderr":        e.String(),
		}
		for i := 0; i < len(extra); i += 2 {
			f[extra[i]] = extra[i+1]
		}
		log.WithFields(f).Error(msg)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		logerr(err, "could not get stdin")
		return err
	}
	defer stdin.Close()

	if err = cmd.Start(); err != nil {
		logerr(err, "failed to start process")
		return err
	}

	in := bufio.NewWriter(stdin)
	for _, l := range script {
		if _, err = in.WriteString(l + "\n"); err != nil {
			logerr(err, "failed to send script line",
				"current-line", l)
			return err
		}
		if err = in.Flush(); err != nil {
			logerr(err, "failed to execute script line",
				"current-line", l)
			return err
		}
	}
	stdin.Close()

	if err = cmd.Wait(); err != nil {
		logerr(err, "unexpected termination")
		return err
	}
	return nil
}

// rsync -e 'ssh -oControlPath=...' ...
func (c *sshMaster) rsync(included, excluded, rargs []string) error {
	c.wg.Add(1)
	defer c.wg.Done()

	args := append(rargs, "-e", fmt.Sprintf("ssh -S %s -p %d -x", c.controlPath, c.port))
	args = append(args, rsyncFilter(included, excluded)...)
	args = append(args, fmt.Sprintf("%s@%s", c.user, c.host))
	cmd := osexec.Command("rsync", args...)

}

// rsyncFilter builds filter arguments for rsync. This is modelled after BackupPC:
// https://github.com/backuppc/backuppc/blob/master/lib/BackupPC/Xfer/Rsync.pm#L234
//
// Original comments are marked as quote ("//>").
//
// TODO: could be simplified.
func rsyncFilter(included, excluded []string) (list []string) {
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

	for _, incl := range included {
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
	for _, f := range excluded {
		//> just append additional exclude lists onto the end
		list = append(list, "--exclude="+f)
	}

	return
}
