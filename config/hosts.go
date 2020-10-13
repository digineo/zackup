package config

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// HostConfigs maps hostnames to config.
type HostConfigs map[string]*JobConfig

func pathToHostVariantA(name string) string {
	return path.Base(path.Dir(name))
}

func pathToHostVariantB(name string) string {
	return strings.TrimSuffix(path.Base(name), ".yml")
}

func (h HostConfigs) readGlob(pattern string, matchToHost func(string) string) (err error) {
	glob, err := filepath.Glob(pattern)
	if err != nil {
		err = errors.Wrapf(err, "expanding glob %q failed", pattern)
		return
	}

	for _, match := range glob {
		host := matchToHost(match)
		if err = h.readHostConfig(host, match); err != nil {
			return
		}
	}

	return
}

func (h HostConfigs) readHostConfig(host, file string) (err error) {
	if _, ok := h[host]; ok {
		err = errors.Errorf("duplicate host config for %s found", host)
		return
	}
	var f *os.File
	if f, err = os.Open(file); err != nil {
		return
	}
	defer f.Close()

	j := JobConfig{}
	if err = yaml.NewDecoder(f).Decode(&j); err != nil {
		err = errors.Wrapf(err, "reading %q", file)
		return
	}
	j.host = host
	h[host] = &j

	return
}

func (h HostConfigs) readHooks(root string) error {
	for host, job := range h {
		if err := job.PreScript.readFiles(root, host, "pre.*.sh"); err != nil {
			return err
		}
		if err := job.PostScript.readFiles(root, host, "post.*.sh"); err != nil {
			return err
		}
	}
	return nil
}
