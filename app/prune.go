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
	Ds   string    // Snapshot dataset name "backups/foo@RFC3339"
	Time time.Time // Parsed timestamp from the dataset name
}

func PruneSnapshots(job *config.JobConfig) {
	var host = job.Host()

	// Set defaults if config is not set
	if job.Retention == nil {
		job.Retention = &config.RetentionConfig{
			Daily:   100000,
			Weekly:  100000,
			Monthly: 100000,
			Yearly:  100000,
		}
	}

	// This catches any gaps in the config
	if job.Retention.Daily == 0 {
		job.Retention.Daily = 100000
	}
	if job.Retention.Weekly == 0 {
		job.Retention.Weekly = 100000
	}
	if job.Retention.Monthly == 0 {
		job.Retention.Monthly = 100000
	}
	if job.Retention.Yearly == 0 {
		job.Retention.Yearly = 100000
	}

	// FIXME probably should iterate over a list instead here
	for _, snapshot := range listKeepers(host, "daily", job.Retention.Daily) {
		log.WithFields(logrus.Fields{
			"snapshot": snapshot,
			"period":   "daily",
		}).Debug("keeping snapshot")
	}
	for _, snapshot := range listKeepers(host, "weekly", job.Retention.Weekly) {
		log.WithFields(logrus.Fields{
			"snapshot": snapshot,
			"period":   "weekly",
		}).Debug("keeping snapshot")
	}
	for _, snapshot := range listKeepers(host, "monthly", job.Retention.Monthly) {
		log.WithFields(logrus.Fields{
			"snapshot": snapshot,
			"period":   "monthly",
		}).Debug("keeping snapshot")
	}
	for _, snapshot := range listKeepers(host, "yearly", job.Retention.Yearly) {
		log.WithFields(logrus.Fields{
			"snapshot": snapshot,
			"period":   "yearly",
		}).Debug("keeping snapshot")
	}

	// TODO subtract keepers from the list of snapshots and rm -rf them
}

// listKeepers returns a list of snapshot that are not subject to deletion
// for a given host, pattern, and keep_count.
func listKeepers(host string, pattern string, keep_count uint) []snapshot {
	var keepers []snapshot
	var last string

	for _, snapshot := range listSnapshots(host) {
		var period string

		// Weekly is special because golang doesn't have support for "week number in year"
		// in Time.Format strings.
		if pattern == "weekly" {
			year, week := snapshot.Time.Local().ISOWeek()
			period = fmt.Sprintf("%d-%d", year, week)
		} else {
			period = snapshot.Time.Local().Format(patterns[pattern])
		}

		if period != last {
			last = period
			keepers = append(keepers, snapshot)

			if uint(len(keepers)) == keep_count {
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
			Ds:   ss,
			Time: ts,
		})
	}

	// ZFS list _should_ be in chronological order but just in case ...
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Time.After(snapshots[j].Time)
	})

	return snapshots
}
