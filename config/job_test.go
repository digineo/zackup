package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func uintp(u uint) *uint { return &u }

func TestMergeConfigSSH(t *testing.T) {
	defaultConf := func() *SSHConfig {
		return &SSHConfig{
			User:    "root",
			Port:    22,
			Timeout: uintp(5),
		}
	}

	tests := map[string]struct {
		victim   *SSHConfig
		expected *SSHConfig
	}{
		"empty": {
			nil,
			defaultConf(),
		},
		"port": {
			&SSHConfig{Port: 2222},
			&SSHConfig{"root", 2222, uintp(5)},
		},
		"user": {
			&SSHConfig{User: "user"},
			&SSHConfig{"user", 22, uintp(5)},
		},
		"timeout0": {
			&SSHConfig{Timeout: uintp(0)},
			&SSHConfig{"root", 22, uintp(0)},
		},
		"timeout10": {
			&SSHConfig{Timeout: uintp(10)},
			&SSHConfig{"root", 22, uintp(10)},
		},
	}

	for name := range tests {
		tc := tests[name]
		t.Run(name, func(t *testing.T) {
			actual := &JobConfig{SSH: tc.victim}
			actual.mergeGlobals(&JobConfig{
				SSH: defaultConf(),
			})
			assert.New(t).Equal(tc.expected, actual.SSH)
		})
	}
}

func TestMergeConfigRSync(t *testing.T) { //nolint:funlen
	defaultConf := func() *RsyncConfig {
		return &RsyncConfig{
			Included:  []string{"/home"},
			Excluded:  []string{"/var/log"},
			Arguments: []string{"--foo"},
		}
	}

	tests := map[string]struct {
		victim   *RsyncConfig
		expected *RsyncConfig
	}{
		"empty": {
			nil,
			defaultConf(),
		},
		"include": {
			&RsyncConfig{
				Included: []string{"/opt"},
			},
			&RsyncConfig{
				Included:  []string{"/opt", "/home"},
				Excluded:  []string{"/var/log"},
				Arguments: []string{"--foo"},
			},
		},
		"include.override": {
			&RsyncConfig{
				Included: []string{"/opt"},

				OverrideGlobalInclude: true,
			},
			&RsyncConfig{
				Included:  []string{"/opt"},
				Excluded:  []string{"/var/log"},
				Arguments: []string{"--foo"},

				OverrideGlobalInclude: true,
			},
		},
		"exclude": {
			&RsyncConfig{
				Excluded: []string{"*.log"},
			},
			&RsyncConfig{
				Included:  []string{"/home"},
				Excluded:  []string{"*.log", "/var/log"},
				Arguments: []string{"--foo"},
			},
		},
		"exclude.override": {
			&RsyncConfig{
				Excluded: []string{"/"},

				OverrideGlobalExclude: true,
			},
			&RsyncConfig{
				Included:  []string{"/home"},
				Excluded:  []string{"/"},
				Arguments: []string{"--foo"},

				OverrideGlobalExclude: true,
			},
		},
		"args": {
			&RsyncConfig{
				Arguments: []string{"--bar"},
			},
			&RsyncConfig{
				Included:  []string{"/home"},
				Excluded:  []string{"/var/log"},
				Arguments: []string{"--bar", "--foo"},
			},
		},
		"args.override": {
			&RsyncConfig{
				Arguments: []string{"--bar"},

				OverrideGlobalArguments: true,
			},
			&RsyncConfig{
				Included:  []string{"/home"},
				Excluded:  []string{"/var/log"},
				Arguments: []string{"--bar"},

				OverrideGlobalArguments: true,
			},
		},
	}

	for name := range tests {
		tc := tests[name]
		t.Run(name, func(t *testing.T) {
			actual := &JobConfig{RSync: tc.victim}
			actual.mergeGlobals(&JobConfig{
				RSync: defaultConf(),
			})
			assert.New(t).Equal(tc.expected, actual.RSync)
		})
	}
}

func TestMergeConfigScripts(t *testing.T) {
	defaultConf := func() Script {
		return Script{
			inline:  []string{"echo global inline"},
			scripts: []string{"echo global scripts"},
		}
	}

	tests := map[string]struct {
		victim   Script
		expected Script
	}{
		"empty": {
			Script{},
			defaultConf(),
		},
		"inline": {
			Script{inline: []string{"echo local inline"}},
			Script{
				inline:  []string{"echo global inline", "echo local inline"},
				scripts: []string{"echo global scripts"},
			},
		},
		"scripts": {
			Script{scripts: []string{"echo local scripts"}},
			Script{
				inline:  []string{"echo global inline"},
				scripts: []string{"echo global scripts", "echo local scripts"},
			},
		},
	}

	for name := range tests {
		tc := tests[name]
		t.Run(name+".pre", func(t *testing.T) {
			actual := &JobConfig{PreScript: tc.victim}
			actual.mergeGlobals(&JobConfig{
				PreScript: defaultConf(),
			})
			assert.New(t).Equal(tc.expected, actual.PreScript)
			assert.New(t).Equal(Script{}, actual.PostScript)
		})
		t.Run(name+".post", func(t *testing.T) {
			actual := &JobConfig{PostScript: tc.victim}
			actual.mergeGlobals(&JobConfig{
				PostScript: defaultConf(),
			})
			assert.New(t).Equal(Script{}, actual.PreScript)
			assert.New(t).Equal(tc.expected, actual.PostScript)
		})
	}
}
