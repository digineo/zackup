package config

// JobConfig holds config settings for a single backup job.
type JobConfig struct {
	host string

	SSH   *SSHConfig   `yaml:"ssh"`
	RSync *RsyncConfig `yaml:"rsync"`

	PreScript  Script `yaml:"pre_script"`  // from yaml file
	PostScript Script `yaml:"post_script"` // from yaml file
}

// SSHConfig holds connection parameters.
type SSHConfig struct {
	User    string `yaml:"user"`    // defaults to "root"
	Port    uint16 `yaml:"port"`    // defaults to 22
	Timeout *uint  `yaml:"timeout"` // number of seconds, defaults to 15
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

	// globals.PreScript
	j.PreScript.inline = append(globals.PreScript.inline, j.PreScript.inline...)
	j.PreScript.scripts = append(globals.PreScript.scripts, j.PreScript.scripts...)

	// globals.PostScript
	j.PostScript.inline = append(globals.PostScript.inline, j.PostScript.inline...)
	j.PostScript.scripts = append(globals.PostScript.scripts, j.PostScript.scripts...)
}
