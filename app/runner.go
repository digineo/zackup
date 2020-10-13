package app

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/digineo/zackup/config"
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
func PerformBackup(job *config.JobConfig) {
	host := job.Host()
	l := log.WithField("job", host)
	var err error

	l.Info("creating dataset")
	ds := newDataset(host)
	if err = ds.create(); err != nil {
		return
	}

	// requires dataset to exist
	defer func() {
		if err == nil {
			l.Info("backup succeeded")
			state.success(host)
			return
		}
		l.WithError(err).Error("backup failed")
		state.failure(host)
	}()
	state.start(host)

	l.Info("establishing SSH tunnel")
	m := newSSHMaster(host, job.SSH)
	if err = m.connect(); err != nil {
		return
	}
	defer m.close()

	if script := job.PreScript.Lines(); len(script) > 0 {
		l.Info("executing pre-scripts")
		if err = m.execute(script); err != nil {
			return
		}
	}

	l.Info("starting rsync")
	if err = m.rsync(job.RSync); err != nil {
		return
	}

	if script := job.PostScript.Lines(); len(script) > 0 {
		l.Info("executing post-scripts")
		if err = m.execute(script); err != nil {
			return
		}
	}

	l.Info("creating snapshot")
	if err = ds.snapshot(); err != nil {
		return
	}
}

// zfs create -p ds.Name.
func (ds *dataset) create() error {
	if err := zfs("create", "-p", ds.Name); err != nil {
		return errors.Wrapf(err, "failed to zfs create %q", ds.Name)
	}
	return nil
}

// zfs snapshot ds.Name@time.RFC3339.
func (ds *dataset) snapshot() error {
	now := time.Now().UTC()
	name := fmt.Sprintf("%s@%s", ds.Name, now.Format(time.RFC3339))

	if err := zfs("snapshot", name); err != nil {
		return errors.Wrapf(err, "failed to zfs snapshot %q", name)
	}
	return nil
}

func zfs(args ...string) error {
	o, e, err := execZFS(args...)
	if err != nil {
		f := appendStdlogs(logrus.Fields{
			logrus.ErrorKey: err,
			"prefix":        "zfs",
			"command":       append([]string{"zfs"}, args...),
		}, o, e)
		log.WithFields(f).Errorf("executing zfs failed")
	}
	return err
}

func execZFS(args ...string) (stdout, stderr *bytes.Buffer, err error) {
	cmd := exec.Command("zfs", args...)

	var o, e bytes.Buffer
	cmd.Stdout = &o
	cmd.Stderr = &e

	return &o, &e, cmd.Run()
}

func appendStdlogs(f logrus.Fields, out, err *bytes.Buffer) logrus.Fields {
	if out != nil && out.Len() > 0 {
		f["stdout"] = out.String()
		out.Reset()
	}
	if err != nil && err.Len() > 0 {
		f["stderr"] = err.String()
		err.Reset()
	}
	return f
}
