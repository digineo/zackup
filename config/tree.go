package config

import (
	"os"
	"path"
	"sort"
	"sync"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Tree is the internal representation of the configuration directory
// passed in as ROOT_DIR flag/env var.
type Tree interface {
	// SetRoot updates the root config directory. If the given path
	// is different than the previously setup path, the config tree is
	// reloaded.
	SetRoot(newRoot string) error

	// Root returns the path to the root config directory. Might be empty.
	Root() string

	// GetHosts returns the list of configured hosts.
	Hosts() []string

	// GetHost returns a copy of the job description for a single host.
	// If the host is unknown, this returns nil.
	Host(name string) *JobConfig

	// Service returns a copy of the current service configuration.
	Service() *ServiceConfig
}

type tree struct {
	root string

	service *ServiceConfig
	global  *JobConfig
	hosts   HostConfigs

	sync.RWMutex
}

// NewTree returns an empty Tree.
func NewTree(root string) Tree {
	return &tree{root: root}
}

func (t *tree) SetRoot(newRoot string) error {
	t.RLock()
	currentRoot := t.root
	t.RUnlock()

	if currentRoot != newRoot {
		t.Lock()
		defer t.Unlock()

		t.root = newRoot
		return t.reload()
	}
	return nil
}

func (t *tree) Root() string {
	return t.root
}

func (t *tree) Service() *ServiceConfig {
	t.RLock()
	defer t.RUnlock()

	if t.service == nil {
		return nil
	}

	dup := *t.service
	return &dup
}

func (t *tree) Hosts() []string {
	t.RLock()
	res := make([]string, 0, len(t.hosts))
	for host := range t.hosts {
		res = append(res, host)
	}
	t.RUnlock()
	sort.Strings(res)
	return res
}

func (t *tree) Host(name string) *JobConfig {
	t.RLock()
	defer t.RUnlock()
	if job, ok := t.hosts[name]; ok {
		j := &JobConfig{host: job.host}
		j.mergeGlobals(job)
		return j
	}
	return nil
}

// Reload (re-) reads the Tree.Root directory into memory.
func (t *tree) Reload() error {
	t.Lock()
	defer t.Unlock()
	return t.reload()
}

func (t *tree) reload() error {
	if t.root == "" {
		t.root = DefaultRoot
	}

	// read service config
	t.service = &ServiceConfig{}
	if err := t.decodeYaml("config.yml", t.service); err != nil {
		return errors.Wrap(err, "failed to load config.yml")
	}

	// read global config
	t.global = &JobConfig{}
	if err := t.decodeYaml("globals.yml", t.global); err != nil {
		return errors.Wrap(err, "failed to load globals.yml")
	}

	// read host configs
	t.hosts = make(HostConfigs)
	if err := t.hosts.readGlob(path.Join(t.root, "hosts/*/config.yml"), pathToHostVariantA); err != nil {
		return err
	}
	if err := t.hosts.readGlob(path.Join(t.root, "hosts/*.yml"), pathToHostVariantB); err != nil {
		return err
	}
	if err := t.hosts.readHooks(t.root); err != nil {
		return err
	}

	// merge global config into host configs
	for _, job := range t.hosts {
		job.mergeGlobals(t.global)
	}

	return nil
}

func (t *tree) decodeYaml(name string, v interface{}) error {
	if !path.IsAbs(name) {
		name = path.Join(t.root, name)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	return yaml.NewDecoder(f).Decode(v)
}
