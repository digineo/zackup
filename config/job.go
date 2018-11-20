package config

// JobConfig holds config settings for a single backup job.
type JobConfig struct {
	SSH   *sshConfig   `yaml:"ssh"`
	RSync *rsyncConfig `yaml:"rsync"`

	PreScript  Script `yaml:"pre_script"`  // from yaml file
	PostScript Script `yaml:"post_script"` // from yaml file

}

type sshConfig struct {
	User     string `yaml:"user"`
	Port     uint16 `yaml:"port"`
	Identity string `yaml:"identity_file"`
}

type rsyncConfig struct {
	Included  []string `yaml:"included"`
	Excluded  []string `yaml:"excluded"`
	Arguments []string `yaml:"args"`
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
			if tmp := globals.SSH.Identity; tmp != "" {
				j.SSH.Identity = tmp
			}
		}
	}

	if globals.RSync != nil {
		if j.RSync == nil {
			dup := *globals.RSync
			j.RSync = &dup
		} else {
			j.RSync.Included = append(j.RSync.Included, globals.RSync.Included...)
			j.RSync.Excluded = append(j.RSync.Excluded, globals.RSync.Excluded...)
		}
	}

	// globals.PreScript
	if tmp := globals.PreScript.inline; len(tmp) > 0 {
		j.PreScript.inline = append(tmp, j.PreScript.inline...)
	}
	if tmp := globals.PreScript.scripts; len(tmp) > 0 {
		j.PreScript.scripts = append(tmp, j.PreScript.scripts...)
	}

	// globals.PostScript
	if tmp := globals.PostScript.inline; len(tmp) > 0 {
		j.PostScript.inline = append(tmp, j.PostScript.inline...)
	}
	if tmp := globals.PostScript.scripts; len(tmp) > 0 {
		j.PostScript.scripts = append(tmp, j.PostScript.scripts...)
	}
}
