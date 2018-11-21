package app

import (
	"bufio"
	"bytes"
	"fmt"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"time"

	"git.digineo.de/digineo/zackup/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// RootDataset is the name of the ZFS dataset under which zackup
	// creates per-host datasets.
	RootDataset = "zpool"

	// MountBase is the name of the directory which zackup uses to
	// mount per-host datasets for rsync.
	MountBase = "/zackup"
)

type dataset struct {
	Host  string
	Mount string
	Name  string
}

func newDataset(host string) *dataset {
	return &dataset{
		Host:  host,
		Mount: filepath.Join(MountBase, host),
		Name:  filepath.Join(RootDataset, host),
	}
}

// PerformBackup executes the backup job.
func PerformBackup(host string, job *config.JobConfig) {
	var err error
	defer func() { state.finish(host, err) }()

	state.start(host)
	ds := newDataset(host)

	if err = ds.create(); err != nil {
		return
	}

	if script := job.PreScript.Lines(); len(script) > 0 {
		if err = ssh(host, job.SSH.User, job.SSH.Port, script); err != nil {
			return
		}
	}

	if err = rsync(host, job.RSync.Included, job.RSync.Excluded, job.RSync.Arguments); err != nil {
		return
	}

	if script := job.PostScript.Lines(); len(script) > 0 {
		if err = ssh(host, job.SSH.User, job.SSH.Port, script); err != nil {
			return
		}
	}

	if err = ds.snapshot(); err != nil {
		return
	}
}

// zfs create -p ds.Name
func (ds *dataset) create() error {
	if err := zfs("create", "-p", ds.Name); err != nil {
		return errors.Wrapf(err, "failed to zfs create %q", ds.Name)
	}
	return nil
}

// zfs snapshot ds.Name@time.RFC3339
func (ds *dataset) snapshot() error {
	now := time.Now().UTC()
	name := fmt.Sprintf("%s@%s", ds.Name, now.Format(time.RFC3339))

	if err := zfs("snapshot", name); err != nil {
		return errors.Wrapf(err, "failed to zfs snapshot %q", name)
	}
	return nil
}

// echo script | ssh -l user -p port host /bin/sh -esx
func ssh(host, user string, port uint16, script []string) error {
	args := []string{
		"-l", user,
		"-p", strconv.Itoa(int(port)),
		host,
		"/bin/sh", "-esx",
	}
	cmd := osexec.Command("ssh", args...)

	f := logrus.Fields{
		"prefix":  "ssh",
		"command": append([]string{"ssh"}, args...),
	}

	var o, e bytes.Buffer
	cmd.Stdout = &o
	cmd.Stderr = &e

	stdin, err := cmd.StdinPipe()
	if err != nil {
		f[logrus.ErrorKey] = err
		log.WithFields(f).Error("could not get stdin")
		return err
	}
	defer stdin.Close()

	if err = cmd.Start(); err != nil {
		f[logrus.ErrorKey] = err
		f["stdout"] = o.String()
		f["stderr"] = e.String()
		log.WithFields(f).Error("failed to start process")
		return err
	}

	in := bufio.NewWriter(stdin)
	for _, l := range script {
		if _, err = in.WriteString(l + "\n"); err != nil {
			f[logrus.ErrorKey] = err
			f["stdout"] = o.String()
			f["stderr"] = e.String()
			f["current-line"] = l
			log.WithFields(f).Error("failed to send script line")
			return err
		}

		if err = in.Flush(); err != nil {
			f[logrus.ErrorKey] = err
			f["stdout"] = o.String()
			f["stderr"] = e.String()
			f["current-line"] = l
			log.WithFields(f).Error("failed to execute script line")
			return err
		}
	}
	stdin.Close()

	if err = cmd.Wait(); err != nil {
		f[logrus.ErrorKey] = err
		f["stdout"] = o.String()
		f["stderr"] = e.String()
		log.WithFields(f).Error("script failed")
		return err
	}
	return nil
}

//
func rsync(host string, included, excluded, args []string) error {
	panic("TODO")
}

func zfs(args ...string) error {
	o, e, err := exec("zfs", args...)

	var action string
	if len(args) > 0 {
		action = args[0]
	}

	l := log.WithFields(logrus.Fields{
		"prefix":  "zfs",
		"command": append([]string{"zfs"}, args...),
		"stdout":  o,
		"stderr":  e,
	})

	if err != nil {
		l.WithError(err).Errorf("zfs %s failed", action)
	} else {
		l.Infof("zfs %s succeeded", action)
	}

	return err
}

func exec(prog string, args ...string) (stdout, stderr string, err error) {
	cmd := osexec.Command(prog, args...)

	var o, e bytes.Buffer
	cmd.Stdout = &o
	cmd.Stderr = &e

	err = cmd.Run()
	return o.String(), e.String(), err
}
