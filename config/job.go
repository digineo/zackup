package config

// JobConfig holds config settings for a single backup job.
type JobConfig struct {
	host string

	SSH       *SSHConfig       `yaml:"ssh"`
	RSync     *RsyncConfig     `yaml:"rsync"`
	Retention *RetentionConfig `yaml:"retention"`

	PreScript  Script `yaml:"pre_script"`  // from yaml file
	PostScript Script `yaml:"post_script"` // from yaml file
}

// SSHConfig holds connection parameters.
type SSHConfig struct {
	User    string `yaml:"user"`    // defaults to "root"
	Port    uint16 `yaml:"port"`    // defaults to 22
	Timeout *uint  `yaml:"timeout"` // number of seconds, defaults to 15
}

// RetentionConfig holds backup retention periods
type RetentionConfig struct {
	Daily   uint `yaml:"daily"`   // defaults to 1000000
	Weekly  uint `yaml:"weekly"`  // defaults to 1000000
	Monthly uint `yaml:"monthly"` // defaults to 1000000
	Yearly  uint `yaml:"yearly"`  // defaults to 1000000
}

// Host returns the hostname for this job.
func (j *JobConfig) Host() string {
	return j.host
}

func (j *JobConfig) mergeGlobals(globals *JobConfig) {
	//nolint:nestif
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

	//nolint:nestif
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

	if globals.Retention != nil {
		if j.Retention == nil {
			dup := *globals.Retention
			j.Retention = &dup
		} else {
			if j.Retention.Daily == 0 {
				j.Retention.Daily = globals.Retention.Daily
			}
			if j.Retention.Weekly == 0 {
				j.Retention.Weekly = globals.Retention.Weekly
			}
			if j.Retention.Monthly == 0 {
				j.Retention.Monthly = globals.Retention.Monthly
			}
			if j.Retention.Yearly == 0 {
				j.Retention.Yearly = globals.Retention.Yearly
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
