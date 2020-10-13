package app

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/digineo/zackup/config"
	"github.com/sirupsen/logrus"
)

// State holds metrics, mainly for Prometheus. Upon start it is restored
// from a disk cache and regularly written back.
type State struct {
	hosts map[string]*metrics
	tree  config.Tree
	mu    *sync.RWMutex
}

var state *State

// InitializeState reads the performance metrics stored in the data.
func InitializeState(tree config.Tree) error {
	state = &State{
		hosts: make(map[string]*metrics),
		tree:  tree,
		mu:    &sync.RWMutex{},
	}

	svc := tree.Service()
	RootDataset = svc.RootDataset
	MountBase = svc.MountBase
	if svc.RSyncPath != "" {
		RSyncPath = svc.RSyncPath
	}
	if svc.SSHPath != "" {
		SSHPath = svc.SSHPath
	}

	return state.load()
}

// ExportState dumps the current performance metrics.
func ExportState() []HostMetrics {
	return state.export()
}

type metrics struct {
	job         *config.JobConfig // configured via InitializeState()
	ScheduledAt time.Time         // configured via reschedule()

	// these are properties read from ZFS

	StartedAt                 time.Time
	SucceededAt               *time.Time
	SuccessDuration           time.Duration
	FailedAt                  *time.Time
	FailureDuration           time.Duration
	SpaceUsedBySnapshots      uint64
	SpaceUsedByDataset        uint64
	SpaceUsedByChildren       uint64
	SpaceUsedByRefReservation uint64
	CompressionFactor         float64
}

func (m *metrics) SpaceUsedTotal() uint64 {
	return m.SpaceUsedBySnapshots + m.SpaceUsedByDataset + m.SpaceUsedByChildren + m.SpaceUsedByRefReservation
}

// HostMetrics represents a snapshot of the current metrics for a host.
type HostMetrics struct {
	Host string
	metrics
}

func (s *State) start(host string) {
	t := time.Now().UTC()
	s.mu.Lock()
	if m, ok := s.hosts[host]; ok {
		m.StartedAt = t
	} else {
		s.hosts[host] = &metrics{
			StartedAt: t,
		}
	}
	storeStart(host, t)
	s.mu.Unlock()
}

func (s *State) success(host string) {
	t := time.Now().UTC()
	s.mu.Lock()
	if m, ok := s.hosts[host]; ok {
		m.SucceededAt = &t
		m.SuccessDuration = t.Sub(m.StartedAt).Truncate(time.Millisecond)
		storeResult(host, true, t, m.SuccessDuration)
	}
	s.mu.Unlock()
}

func (s *State) failure(host string) {
	t := time.Now().UTC()
	s.mu.Lock()
	if m, ok := s.hosts[host]; ok {
		m.FailedAt = &t
		m.FailureDuration = t.Sub(m.StartedAt).Truncate(time.Millisecond)
		storeResult(host, false, t, m.FailureDuration)
	}
	s.mu.Unlock()
}

func (s *State) reschedule(host string, t time.Time) {
	s.mu.RLock()
	if job, ok := s.hosts[host]; ok {
		job.ScheduledAt = s.tree.Service().NextSchedule(t)
	}
	s.mu.RUnlock()
}

func (s *State) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	next := s.tree.Service().NextSchedule

	for _, host := range s.tree.Hosts() {
		if _, ok := s.hosts[host]; !ok {
			s.hosts[host] = &metrics{
				ScheduledAt: next(now),
			}
		}
		if job := s.tree.Host(host); job != nil {
			s.hosts[host].job = job
		}
		if err := s.loadHost(host); err != nil {
			return err
		}
	}
	return nil
}

// unsafe, caller must lock s.mu mutex.
func (s *State) loadHost(host string) error { //nolint:funlen
	dataset := filepath.Join(RootDataset, host)
	args := []string{
		"get", "-H", "-p",
		"-t", "filesystem",
		"-s", "local,none",
		"-o", "name,property,value",
		zackupProps,
		dataset,
	}

	o, e, err := execZFS(args...)
	if err != nil {
		// dataset does not exit, ignore
		f := appendStdlogs(logrus.Fields{
			logrus.ErrorKey: err,
			"command":       append([]string{"zfs"}, args...),
		}, o, e)
		log.WithFields(f).Trace("failed to load state")
		return nil
	}

	isTab := func(r rune) bool { return r == '\t' }
	rds := RootDataset + "/"
	scan := bufio.NewScanner(o)

	for scan.Scan() {
		cols := strings.FieldsFunc(scan.Text(), isTab)
		if len(cols) != 3 {
			continue
		}
		l := log.WithFields(logrus.Fields{
			"dataset":  cols[0],
			"propname": cols[1],
			"propval":  cols[2],
		})

		if cols[0] != dataset {
			l.Trace("ignore child dataset")
			continue
		}
		if cols[2] == "-" {
			l.Trace("ignore empty values")
			continue
		}
		decoder, ok := propDecoder[cols[1]]
		if !ok {
			l.Trace("ignore non-zackup property")
			continue
		}

		host := strings.TrimPrefix(cols[0], rds)
		met, ok := s.hosts[host]
		if !ok {
			met = &metrics{}
			s.hosts[host] = met
		}

		if err := decoder(met, cols[2]); err != nil {
			l.WithError(err).Trace("failed to parse value, ignore")
			continue
		}
		l.Trace("accepted line")
	}

	if err := scan.Err(); err != nil {
		log.WithFields(appendStdlogs(logrus.Fields{
			logrus.ErrorKey: err,
			"command":       append([]string{"zfs"}, args...),
		}, o, e)).Error("failed to load state")
		return fmt.Errorf("failed to load state: %w", err)
	}
	return nil
}

func (s *State) export() (ex []HostMetrics) {
	s.mu.RLock()
	ex = make([]HostMetrics, 0, len(s.hosts))

	for host, met := range s.hosts {
		ex = append(ex, HostMetrics{
			Host: host,
			metrics: metrics{
				ScheduledAt:               met.ScheduledAt,
				StartedAt:                 met.StartedAt,
				SucceededAt:               met.SucceededAt,
				SuccessDuration:           met.SuccessDuration,
				FailedAt:                  met.FailedAt,
				FailureDuration:           met.FailureDuration,
				SpaceUsedBySnapshots:      met.SpaceUsedBySnapshots,
				SpaceUsedByDataset:        met.SpaceUsedByDataset,
				SpaceUsedByChildren:       met.SpaceUsedByChildren,
				SpaceUsedByRefReservation: met.SpaceUsedByRefReservation,
				CompressionFactor:         met.CompressionFactor,
			},
		})
	}

	sort.Slice(ex, func(i, j int) bool {
		return ex[i].Host < ex[j].Host
	})

	s.mu.RUnlock()
	return
}

func storeStart(host string, t time.Time) error {
	args := []string{
		"set",
		fmt.Sprintf("%s=%d", propZackupLastStart, t.Unix()),
		filepath.Join(RootDataset, host),
	}

	f := logrus.Fields{
		"command": "zfs",
		"args":    args,
	}
	log.WithFields(f).
		Debugf("set properties for host %q", host)
	o, e, err := execZFS(args...)
	if err != nil {
		f[logrus.ErrorKey] = err
		log.WithFields(appendStdlogs(f, o, e)).
			Error("failed to store start state")
		return err
	}
	return nil
}

func storeResult(host string, success bool, t time.Time, dur time.Duration) error {
	propTime, propDur := propZackupLastFailureDate, propZackupLastFailureDuration
	if success {
		propTime, propDur = propZackupLastSuccessDate, propZackupLastSuccessDuration
	}

	args := []string{
		"set",
		fmt.Sprintf("%s=%d", propTime, t.Unix()),
		fmt.Sprintf("%s=%d", propDur, int64(dur/time.Millisecond)),
		filepath.Join(RootDataset, host),
	}
	f := logrus.Fields{
		"command": "zfs",
		"args":    args,
	}
	log.WithFields(f).Debugf("set properties for host %q", host)
	o, e, err := execZFS(args...)
	if err != nil {
		f[logrus.ErrorKey] = err
		log.WithFields(appendStdlogs(f, o, e)).
			Error("failed to store result state")
		return err
	}
	return nil
}
