package config

import (
	"time"

	"github.com/sirupsen/logrus"
)

// JobConfig holds config settings for a single backup job.
type JobConfig struct {
	host    string
	nextRun time.Time `yaml:"-"` // readonly, will be modified by the scheduler
	active  bool      `yaml:"-"` // readonly, will be modified by the scheduler

	SSH   *SSHConfig   `yaml:"ssh"`
	RSync *RsyncConfig `yaml:"rsync"`

	PreScript  Script `yaml:"pre_script"`  // from yaml file
	PostScript Script `yaml:"post_script"` // from yaml file
}

// SSHConfig holds connection parameters
type SSHConfig struct {
	User    string `yaml:"user"`    // defaults to "root"
	Port    uint16 `yaml:"port"`    // defaults to 22
	Timeout *uint  `yaml:"timeout"` // number of seconds, defaults to 15
}

// Host returns the hostname for this job.
func (j *JobConfig) Host() string {
	return j.host
}

// Start marks the job as active.
func (j *JobConfig) Start() { j.active = true }

// Finish marks a job as done.
func (j *JobConfig) Finish() { j.active = false }

// IsActive returns whether the job is currently running
func (j *JobConfig) IsActive() bool { return j.active }

// Schedule updates the scheduled time for the next run.
func (j *JobConfig) Schedule(next time.Time) {
	logrus.WithFields(logrus.Fields{
		"job":  j.host,
		"date": next.Truncate(time.Second).Format(time.RFC3339),
	}).Info("job rescheduled")
	j.nextRun = next
}

// NextRun returns the next scheduled run time.
func (j *JobConfig) NextRun() time.Time { return j.nextRun }

func (j *JobConfig) mergeGlobals(globals *JobConfig) {
	if globals.SSH != nil {
		if j.SSH == nil {
			dup := *globals.SSH
			j.SSH = &dup
		} else {
			if j.SSH.User == "" {
				j.SSH.User = globals.SSH.User
			}
			if j.SSH.Port == 0 {
				j.SSH.Port = globals.SSH.Port
			}
			if j.SSH.Timeout == nil && globals.SSH.Timeout != nil {
				dup := *globals.SSH.Timeout
				j.SSH.Timeout = &dup
			}
		}
	}

	if globals.RSync != nil {
		if j.RSync == nil {
			dup := *globals.RSync
			j.RSync = &dup
		} else {
			if !j.RSync.OverrideGlobalInclude {
				j.RSync.Included = append(j.RSync.Included, globals.RSync.Included...)
			}
			if !j.RSync.OverrideGlobalExclude {
				j.RSync.Excluded = append(j.RSync.Excluded, globals.RSync.Excluded...)
			}
			if !j.RSync.OverrideGlobalArguments {
				j.RSync.Arguments = append(j.RSync.Arguments, globals.RSync.Arguments...)
			}
		}
	}

	// globals.PreScript
	j.PreScript.inline = append(globals.PreScript.inline, j.PreScript.inline...)
	j.PreScript.scripts = append(globals.PreScript.scripts, j.PreScript.scripts...)

	// globals.PostScript
	j.PostScript.inline = append(globals.PostScript.inline, j.PostScript.inline...)
	j.PostScript.scripts = append(globals.PostScript.scripts, j.PostScript.scripts...)
}
