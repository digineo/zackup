package app

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"git.digineo.de/digineo/zackup/config"
	"github.com/sirupsen/logrus"
)

type sshMaster struct {
	host string
	user string
	port uint16

	controlPath string // SSH multiplexing socket
	mountPath   string // join(MountBase, host)

	tunnel *osexec.Cmd     // foreground SSH process
	wg     *sync.WaitGroup // for execute/rsync

	mu *sync.Mutex // lock for start/stop
}

func newSSHMaster(host string, port uint16, user string) *sshMaster {
	return &sshMaster{
		host: host,
		user: user,
		port: port,

		controlPath: filepath.Join(MountBase, ".zackup", ".%h_%C"),
		mountPath:   filepath.Join(MountBase, host),

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
	}
	cmd := osexec.Command("ssh", args...)

	log.WithFields(logrus.Fields{
		"prefix": "ssh.master",
		"host":   c.host,
		"args":   args,
	}).Info("Starting SSH tunnel")

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
		f := appendStdlogs(logrus.Fields{
			logrus.ErrorKey: err,
			"prefix":        "ssh.execute",
			"host":          c.host,
		}, &o, &e)

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
func (c *sshMaster) rsync(r *config.RsyncConfig) error {
	c.wg.Add(1)
	defer c.wg.Done()

	l := log.WithFields(logrus.Fields{
		"prefix": "ssh.rsync",
		"host":   c.host,
	})

	args := r.BuildArgVector(
		/* ssh */ fmt.Sprintf("ssh -S %s -p %d -x", c.controlPath, c.port),
		/* src */ fmt.Sprintf("%s@%s:", c.user, c.host),
		/* dst */ c.mountPath,
	)

	cmd := osexec.Command("rsync", args...)

	done, wg, err := captureOutput(l, cmd)
	if err != nil {
		return err
	}
	defer done()

	l.WithField("args", args).Info("Starting rsync")
	if err := cmd.Start(); err != nil {
		return err
	}
	wg.Wait()
	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func captureOutput(log *logrus.Entry, cmd *osexec.Cmd) (func(), *sync.WaitGroup, error) {
	wg := &sync.WaitGroup{}

	capture := func(name string, r io.Reader) {
		caplog := log.WithField("stream", name)
		s := bufio.NewScanner(r)
		for s.Scan() {
			caplog.Info(s.Text())
		}
		if err := s.Err(); err != nil {
			caplog.WithError(err).Error("unexpected end of stream")
		}
		wg.Done()
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdout.Close()
		return nil, nil, err
	}

	wg.Add(2)
	go capture("stdout", stdout)
	go capture("stderr", stderr)

	return func() {
		stderr.Close()
		stdout.Close()
	}, wg, nil
}
