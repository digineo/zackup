package config

import (
	"bufio"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// Script is a set of script lines.
type Script struct {
	inline  []string // from host yaml
	scripts []string // from files
}

// Lines returns the combined inline and file script lines (in that order).
func (s *Script) Lines() []string {
	buf := make([]string, 0, len(s.inline)+len(s.scripts))
	buf = append(buf, s.inline...)
	buf = append(buf, s.scripts...)
	return buf
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (s *Script) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var inline string
	if err := unmarshal(&inline); err != nil {
		return err
	}

	r := strings.NewReader(inline)
	lines, err := cleanScript(r)
	if err != nil {
		return err
	}

	*s = Script{inline: lines}
	return nil
}

func (s *Script) readFiles(root, host, pattern string) error {
	glob, err := filepath.Glob(path.Join(root, host, pattern))
	if err != nil {
		return err
	}

	sort.Strings(glob)
	for _, file := range glob {
		lines, err := readScriptFile(file)
		if err != nil {
			return err
		}

		s.scripts = append(s.scripts, lines...)
	}
	return nil
}

func readScriptFile(name string) ([]string, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	lines, err := cleanScript(f)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse %s", name)
	}
	return lines, nil
}

func cleanScript(r io.Reader) (cleaned []string, err error) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || line[0] == '#' {
			// ignore empty lines and comments
			continue
		}
		cleaned = append(cleaned, line)
	}
	err = s.Err()
	return
}
