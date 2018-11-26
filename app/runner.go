package app

import (
	"bytes"
	"fmt"
	osexec "os/exec"
	"path/filepath"
	"time"

	"git.digineo.de/digineo/zackup/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

	m := newSSHMaster(host, job.SSH.Port, job.SSH.User)
	if err = m.connect(); err != nil {
		return
	}
	defer m.close()

	if script := job.PreScript.Lines(); len(script) > 0 {
		if err = m.execute(script); err != nil {
			return
		}
	}

	if err = m.rsync(job.RSync); err != nil {
		return
	}

	if script := job.PostScript.Lines(); len(script) > 0 {
		if err = m.execute(script); err != nil {
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
