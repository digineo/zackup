package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/digineo/zackup/config"
	"github.com/sirupsen/logrus"
)

type sshMaster struct {
	host string
	user string
	port uint16

	connectTimeout uint   // number of seconds
	controlPath    string // SSH multiplexing socket
	mountPath      string // join(MountBase, host)

	tunnel *exec.Cmd       // foreground SSH process
	wg     *sync.WaitGroup // for execute/rsync

	mu *sync.Mutex // lock for start/stop
}

func newSSHMaster(host string, cfg *config.SSHConfig) *sshMaster {
	master := &sshMaster{
		host: host,
		user: cfg.User,
		port: cfg.Port,

		controlPath: filepath.Join(MountBase, ".zackup_%h_%C"),
		mountPath:   filepath.Join(MountBase, host),

		mu: &sync.Mutex{},
		wg: &sync.WaitGroup{},
	}
	if master.port == 0 {
		master.port = 22
	}
	if master.user == "" {
		master.user = "root"
	}
	if to := cfg.Timeout; to != nil {
		master.connectTimeout = *to
	}
	return master
}

var ErrAlreadyConnected = errors.New("ssh: tunnel already established")

// ssh user@host -p port -o ControlMaster=yes -o ControlPath=...
func (c *sshMaster) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tunnel != nil {
		return ErrAlreadyConnected
	}

	args := []string{
		"-S", c.controlPath, // == -oControlPath=...
		"-o", "ControlMaster=yes",
		"-o", "StrictHostKeyChecking=yes", // default=ask (prompt)
	}
	if c.connectTimeout > 0 {
		args = append(args, "-o", fmt.Sprintf("ConnectTimeout=%d", c.connectTimeout))
	}
	args = append(args,
		"-n", // disable stdin
		"-N", // do not execute command on remote server
		"-T", // disable TTY allocation
		"-x", // disable X11 forwarding
		"-p", strconv.Itoa(int(c.port)),
		"-l", c.user,
		c.host,
	)
	cmd := exec.Command(SSHPath, args...)

	log.WithFields(logrus.Fields{
		"prefix": "ssh.master",
		"job":    c.host,
		"args":   args,
	}).Debug("Starting SSH tunnel")

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ssh: could not establish tunnel: %w", err)
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
		"job":    c.host,
	})

	// wait for running commands to finish
	c.wg.Wait()

	if err := c.tunnel.Process.Signal(syscall.SIGTERM); err != nil {
		l.WithError(err).Warn("closing with interrupt failed, send kill")
		time.Sleep(500 * time.Millisecond)
		if err = c.tunnel.Process.Kill(); err != nil {
			l.WithError(err).Error("closing with kill failed")
		}
	}

	if err := c.tunnel.Wait(); err != nil {
		// since we've sig{term,kill}'ed the process, we're not interested
		// in the ExitError
		var xit exec.ExitError
		if !errors.Is(err, &xit) {
			l.WithError(err).Warn("unexpected termination")
		}
	}

	c.tunnel = nil
}

// execute a script on the remote host:
//	echo script | ssh -oControlPath=... host /bin/sh -esx
func (c *sshMaster) execute(script []string) error { //nolint:funlen
	c.wg.Add(1)
	defer c.wg.Done()

	args := []string{
		"-S", c.controlPath, // == -oControlPath=...
		"-o", "ControlMaster=yes",
		"-o", "StrictHostKeyChecking=yes", // default=ask (prompt)
	}
	if c.connectTimeout > 0 {
		args = append(args, "-o", fmt.Sprintf("ConnectTimeout=%d", c.connectTimeout))
	}
	args = append(args,
		"-p", strconv.Itoa(int(c.port)),
		"-x", // disable X11 forwarding
		"-l", c.user,
		c.host,
		"/bin/sh", "-esx",
	)
	cmd := exec.Command(SSHPath, args...)

	l := log.WithFields(logrus.Fields{
		"prefix": "ssh.execute",
		"job":    c.host,
	})

	done, wg, err := captureOutput(l, cmd)
	if err != nil {
		return err
	}
	defer done()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		l.WithError(err).Error("could not get stdin")
		return fmt.Errorf("ssh: could not get stdin: %w", err)
	}
	defer stdin.Close()

	if err = cmd.Start(); err != nil {
		l.WithError(err).Error("failed to start process")
		return fmt.Errorf("ssh: failed to start process: %w", err)
	}

	in := bufio.NewWriter(stdin)
	for _, line := range script {
		if _, err = in.WriteString(line + "\n"); err != nil {
			l.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"current-line":  line,
			}).Error("failed to send script")
			return fmt.Errorf("ssh: failed to send script: %w", err)
		}
		if err = in.Flush(); err != nil {
			l.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"current-line":  line,
			}).Error("failed to execute script")
			return fmt.Errorf("ssh: failed to execute script: %w", err)
		}
	}
	stdin.Close()
	wg.Wait()

	if err = cmd.Wait(); err != nil {
		l.WithError(err).Error("unexpected termination")
		return fmt.Errorf("ssh: unexpected termination: %w", err)
	}
	return nil
}

// rsync -e 'ssh -oControlPath=...' ...
func (c *sshMaster) rsync(r *config.RsyncConfig) error {
	c.wg.Add(1)
	defer c.wg.Done()

	l := log.WithFields(logrus.Fields{
		"prefix": "ssh.rsync",
		"job":    c.host,
	})

	sshArg := fmt.Sprintf("ssh -S %s -p %d -x -oStrictHostKeyChecking=yes", c.controlPath, c.port)
	if c.connectTimeout > 0 {
		sshArg += fmt.Sprintf(" -oConnectTimeout=%d", c.connectTimeout)
	}
	srcArg := fmt.Sprintf("%s@%s:", c.user, c.host)

	args := r.BuildArgVector(sshArg, srcArg, c.mountPath)
	cmd := exec.Command(RSyncPath, args...)

	done, wg, err := captureOutput(l, cmd)
	if err != nil {
		return err
	}
	defer done()

	if err := cmd.Start(); err != nil {
		return err //nolint:wrapcheck
	}
	wg.Wait()
	if err := cmd.Wait(); err != nil {
		return err //nolint:wrapcheck
	}

	return nil
}

func captureOutput(log *logrus.Entry, cmd *exec.Cmd) (func(), *sync.WaitGroup, error) {
	wg := &sync.WaitGroup{}

	capture := func(name string, r io.Reader) {
		caplog := log.WithField("stream", name)
		s := bufio.NewScanner(r)
		for s.Scan() {
			caplog.Trace(s.Text())
		}
		if err := s.Err(); err != nil {
			caplog.WithError(err).Error("unexpected end of stream")
		}
		wg.Done()
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err //nolint:wrapcheck
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdout.Close()
		return nil, nil, err //nolint:wrapcheck
	}

	wg.Add(2)
	go capture("stdout", stdout)
	go capture("stderr", stderr)

	return func() {
		stderr.Close()
		stdout.Close()
	}, wg, nil
}
