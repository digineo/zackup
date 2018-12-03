package app

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// State holds metrics, mainly for Prometheus. Upon start it is restored
// from a disk cache and regularly written back.
type State struct {
	results map[string]*metrics
	mu      *sync.RWMutex
}

type metrics struct {
	StartedAt            time.Time
	SucceededAt          *time.Time
	SuccessDuration      time.Duration
	FailedAt             *time.Time
	FailureDuration      time.Duration
	SpaceUsedTotal       uint64
	SpaceUsedBySnapshots uint64
	CompressionFactor    float64
}

// HostMetrics represents a snapshot of the current metrics for a host.
type HostMetrics struct {
	Host string
	metrics
}

// MetricStatus represents the status of a metric set.
type MetricStatus int

// All possible MetricStatus values.
const (
	StatusUnknown MetricStatus = iota
	StatusPrimed
	StatusSuccess
	StatusFailed
	StatusRunning
)

func (s MetricStatus) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusPrimed:
		return "primed"
	case StatusSuccess:
		return "success"
	case StatusFailed:
		return "failed"
	case StatusRunning:
		return "running"
	}
	return fmt.Sprintf("%%!MetricStatus(%d)", s)
}

func (m *metrics) Status() MetricStatus {
	t0, tOK, tErr := m.StartedAt, m.SucceededAt, m.FailedAt

	if t0.IsZero() {
		return StatusPrimed
	}
	if (tOK == nil || t0.After(*tOK)) && (tErr == nil || t0.After(*tErr)) {
		return StatusRunning
	}
	if tOK != nil && (tErr == nil || tOK.After(*tErr)) && !tOK.Before(t0) {
		return StatusSuccess
	}
	if tErr != nil && (tOK == nil || tErr.After(*tOK)) && !tErr.Before(t0) {
		return StatusFailed
	}
	return StatusUnknown
}

var state = &State{
	results: make(map[string]*metrics),
	mu:      &sync.RWMutex{},
}

func (s *State) start(host string) {
	t := time.Now().UTC()
	s.mu.Lock()
	if m, ok := s.results[host]; ok {
		m.StartedAt = t
	} else {
		s.results[host] = &metrics{
			StartedAt: t,
		}
	}
	storeStart(host, t)
	s.mu.Unlock()
}

func (s *State) finish(host string, err error) {
	if err == nil {
		s.success(host)
	} else {
		s.failure(host)
	}
}

func (s *State) success(host string) {
	t := time.Now().UTC()
	s.mu.Lock()
	if m, ok := s.results[host]; ok {
		m.SucceededAt = &t
		m.SuccessDuration = t.Sub(m.StartedAt).Truncate(time.Millisecond)
		storeResult(host, true, t, m.SuccessDuration)
	}
	s.mu.Unlock()
}

func (s *State) failure(host string) {
	t := time.Now().UTC()
	s.mu.Lock()
	if m, ok := s.results[host]; ok {
		m.FailedAt = &t
		m.FailureDuration = t.Sub(m.StartedAt).Truncate(time.Millisecond)
		storeResult(host, false, t, m.FailureDuration)
	}
	s.mu.Unlock()
}

// LoadState reads the performance metrics stored in the data
func LoadState(hosts []string) error {
	return state.load(hosts)
}

// ExportState dumps the current performance metrics.
func ExportState() []HostMetrics {
	return state.export()
}

func (s *State) load(hosts []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, host := range hosts {
		if _, ok := s.results[host]; !ok {
			s.results[host] = &metrics{}
		}
		if err := s.loadHost(host); err != nil {
			return err
		}
	}
	return nil
}

// unsafe, caller must lock s.mu
func (s *State) loadHost(host string) error {
	dataset := filepath.Join(RootDataset, host)
	args := []string{
		"get", "-H", "-p",
		"-t", "filesystem",
		"-s", "local,none",
		"-o", "name,property,value",
		zackupProps,
		dataset,
	}

	o, e, err := exec("zfs", args...)
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
		met, ok := s.results[host]
		if !ok {
			met = &metrics{}
			s.results[host] = met
		}

		if err := decoder(met, cols[2]); err != nil {
			l.WithError(err).Trace("failed to parse value, ignore")
			continue
		}
		l.Trace("accepted line")
	}

	if err := scan.Err(); err != nil {
		f := appendStdlogs(logrus.Fields{
			logrus.ErrorKey: err,
			"command":       append([]string{"zfs"}, args...),
		}, o, e)
		log.WithFields(f).Error("failed to load state")
		return err
	}
	return nil
}

func storeStart(host string, t time.Time) error {
	args := []string{
		"set",
		fmt.Sprintf("%s=%d", propZackupLastStart, t.Unix()),
		filepath.Join(RootDataset, host),
	}

	log.WithField("command", append([]string{"zfs"}, args...)).
		Debugf("set properties for host %q", host)
	o, e, err := exec("zfs", args...)
	if err != nil {
		f := appendStdlogs(logrus.Fields{
			logrus.ErrorKey: err,
			"command":       append([]string{"zfs"}, args...),
		}, o, e)
		log.WithFields(f).Error("failed to store start state")
		return err
	}
	return nil
}

func storeResult(host string, success bool, t time.Time, dur time.Duration) error {
	var propTime, propDur = propZackupLastFailureDate, propZackupLastFailureDuration
	if success {
		propTime, propDur = propZackupLastSuccessDate, propZackupLastSuccessDuration
	}

	args := []string{
		"set",
		fmt.Sprintf("%s=%d", propTime, t.Unix()),
		fmt.Sprintf("%s=%d", propDur, int64(dur/time.Millisecond)),
		filepath.Join(RootDataset, host),
	}
	log.WithField("command", append([]string{"zfs"}, args...)).
		Debugf("set properties for host %q", host)

	o, e, err := exec("zfs", args...)
	if err != nil {
		f := appendStdlogs(logrus.Fields{
			logrus.ErrorKey: err,
			"command":       append([]string{"zfs"}, args...),
		}, o, e)
		log.WithFields(f).Error("failed to store result state")
		return err
	}
	return nil
}

func (s *State) export() (ex []HostMetrics) {
	s.mu.RLock()
	ex = make([]HostMetrics, 0, len(s.results))

	for host, met := range s.results {
		ex = append(ex, HostMetrics{
			Host: host,
			metrics: metrics{
				StartedAt:            met.StartedAt,
				SucceededAt:          met.SucceededAt,
				SuccessDuration:      met.SuccessDuration,
				FailedAt:             met.FailedAt,
				FailureDuration:      met.FailureDuration,
				SpaceUsedTotal:       met.SpaceUsedTotal,
				SpaceUsedBySnapshots: met.SpaceUsedBySnapshots,
				CompressionFactor:    met.CompressionFactor,
			},
		})
	}

	sort.Slice(ex, func(i, j int) bool {
		return ex[i].Host < ex[j].Host
	})

	s.mu.RUnlock()
	return
}
