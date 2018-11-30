package config

// JobConfig holds config settings for a single backup job.
type JobConfig struct {
	host string `yaml:"-"`

	SSH   *sshConfig   `yaml:"ssh"`
	RSync *RsyncConfig `yaml:"rsync"`

	PreScript  Script `yaml:"pre_script"`  // from yaml file
	PostScript Script `yaml:"post_script"` // from yaml file
}

type sshConfig struct {
	User string `yaml:"user"`
	Port uint16 `yaml:"port"`
}

// Host returns the hostname for this job.
func (j *JobConfig) Host() string {
	return j.host
}

func (j *JobConfig) mergeGlobals(globals *JobConfig) {
	if globals.SSH != nil {
		if j.SSH == nil {
			dup := *globals.SSH
			j.SSH = &dup
		} else {
			if tmp := globals.SSH.User; tmp != "" {
				j.SSH.User = tmp
			}
			if tmp := globals.SSH.Port; tmp != 0 {
				j.SSH.Port = tmp
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
