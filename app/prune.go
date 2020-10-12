package app

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/digineo/zackup/config"
	"github.com/sirupsen/logrus"
)

var ErrNoRetentionPolicy = errors.New("no retention policy defined")

type snapshot struct {
	Name string    // Snapshot dataset name "backups/foo@RFC3339"
	Time time.Time // Parsed timestamp from the dataset name
}

// PruneSnapshots fits existing ZFS snapshots against the retention
// policy configured in job, and prunes those not matching.
func PruneSnapshots(job *config.JobConfig) error {
	if job.Retention == nil {
		return ErrNoRetentionPolicy
	}

	var (
		now         = time.Now()
		policy      = newRetentionPolicy(now, job.Retention)
		snapshots   = listSnapshots(job.Host())
		_, toDelete = policy.apply(snapshots)
	)

	// TODO rm -rf these snapshots
	_ = toDelete

	return nil
}

// listSnapshots calls out to ZFS for a list of snapshots for a given host.
// Returned data will be sorted by time, most recent first.
func listSnapshots(host string) []snapshot {
	ds := newDataset(host)

	args := []string{
		"list",
		"-r",         // recursive
		"-H",         // no field headers in output
		"-o", "name", // only name field
		"-t", "snapshot", // type snapshot
		ds.Name,
	}
	o, e, err := execZFS(args...)
	if err != nil {
		f := appendStdlogs(logrus.Fields{
			logrus.ErrorKey: err,
			"prefix":        "zfs",
			"command":       append([]string{"zfs"}, args...),
		}, o, e)
		log.WithFields(f).Errorf("executing zfs list failed")
	}

	out := o.String()
	snapshots := make([]snapshot, 0, strings.Count(out, "\n"))

	for _, f := range strings.Fields(out) {
		ts, err := time.Parse(time.RFC3339, strings.Split(f, "@")[1])
		if err != nil {
			log.WithField("snapshot", f).Error("Unable to parse timestamp from snapshot")
			continue
		}

		snapshots = append(snapshots, snapshot{
			Name: f,
			Time: ts,
		})
	}

	// ZFS list _should_ be in chronological order but just in case ...
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Time.After(snapshots[j].Time)
	})

	return snapshots
}
