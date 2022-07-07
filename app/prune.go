package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/digineo/zackup/config"
	"github.com/sirupsen/logrus"
)

var (
	patterns = map[string]string{
		"daily":   "2006-01-02",
		"weekly":  "", // See special case in keepers()
		"monthly": "2006-01",
		"yearly":  "2006",
	}
)

type snapshot struct {
	Name string    // Snapshot dataset name "backups/foo@RFC3339"
	Time time.Time // Parsed timestamp from the dataset name
}

// FIXME PruneSnapshots does not actually perform any destructive operations
// on your datasets at this time.
func PruneSnapshots(job *config.JobConfig) {
	var host = job.Host()

	// Defaults: if config is not set
	if job.Retention == nil {
		job.Retention = &config.RetentionConfig{
			Daily:   nil,
			Weekly:  nil,
			Monthly: nil,
			Yearly:  nil,
		}
	}

	var policies = map[string]*int{
		"daily":   job.Retention.Daily,
		"weekly":  job.Retention.Weekly,
		"monthly": job.Retention.Monthly,
		"yearly":  job.Retention.Yearly,
	}

	snapshots := listSnapshots(host)

	for bucket, retention := range policies {
		for _, snapshot := range listKeepers(snapshots, bucket, retention) {
			l := log.WithFields(logrus.Fields{
				"snapshot": snapshot,
				"bucket":   bucket,
			})

			if retention == nil {
				l = l.WithField("retention", "infinite")
			} else {
				l = l.WithField("retention", *retention)
			}

			l.Debug("keeping snapshot")
		}
	}

	// TODO subtract keepers from the list of snapshots and rm -rf them
}

// listKeepers returns a list of snapshot that are not subject to deletion
// for a given host, pattern, and retention.
func listKeepers(snapshots []snapshot, bucket string, retention *int) []snapshot {
	var keepers []snapshot
	var last string

	for _, snapshot := range snapshots {
		var period string

		// Weekly is special because golang doesn't have support for "week number in year"
		// as Time.Format string pattern.
		if bucket == "weekly" {
			year, week := snapshot.Time.Local().ISOWeek()
			period = fmt.Sprintf("%d-%d", year, week)
		} else {
			period = snapshot.Time.Local().Format(patterns[bucket])
		}

		if period != last {
			last = period
			keepers = append(keepers, snapshot)

			// nil will keep infinite snapshots
			if retention == nil {
				continue
			}

			if len(keepers) == *retention {
				break
			}
		}
	}

	return keepers
}

// listSnapshots calls out to ZFS for a list of snapshots for a given host.
// Returned data will be sorted by time, most recent first.
func listSnapshots(host string) []snapshot {
	var snapshots []snapshot

	ds := newDataset(host)

	args := []string{
		"list",
		"-r",         // recursive
		"-H",         // no field headers in output
		"-o", "name", // only name field
		"-t", "snapshot", // type snapshot
		ds.Name,
	}
	o, e, err := execProgram("zfs", args...)
	if err != nil {
		f := appendStdlogs(logrus.Fields{
			logrus.ErrorKey: err,
			"prefix":        "zfs",
			"command":       append([]string{"zfs"}, args...),
		}, o, e)
		log.WithFields(f).Errorf("executing zfs list failed")
	}

	for _, ss := range strings.Fields(o.String()) {
		ts, err := time.Parse(time.RFC3339, strings.Split(ss, "@")[1])

		if err != nil {
			log.WithField("snapshot", ss).Error("Unable to parse timestamp from snapshot")
			continue
		}

		snapshots = append(snapshots, snapshot{
			Name: ss,
			Time: ts,
		})
	}

	// ZFS list _should_ be in chronological order but just in case ...
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Time.After(snapshots[j].Time)
	})

	return snapshots
}
