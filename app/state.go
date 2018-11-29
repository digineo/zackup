package app

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
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
	StartedAt       time.Time     `json:"started_at,omitempty"`
	SucceededAt     *time.Time    `json:"succeeded_at,omitempty"`
	SuccessDuration time.Duration `json:"success_duration,omitempty"`
	FailedAt        *time.Time    `json:"failed_at,omitempty"`
	FailureDuration time.Duration `json:"failure_duration,omitempty"`
}

// HostMetrics represents a snapshot of the current metrics for a host.
type HostMetrics struct {
	Host string `json:"host"`
	metrics
}

// MetricStatus represents the status of a metric set.
type MetricStatus int

// All possible MetricStatus values.
const (
	StatusUnknown MetricStatus = iota
	StatusSuccess
	StatusFailed
	StatusRunning
)

func (s MetricStatus) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
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
		return StatusUnknown
	}
	if (tOK == nil || t0.After(*tOK)) && (tErr == nil || t0.After(*tErr)) {
		return StatusRunning
	}
	if tOK != nil && (tErr == nil || tOK.After(*tErr)) && tOK.After(t0) {
		return StatusSuccess
	}
	if tErr != nil && (tOK == nil || tErr.After(*tOK)) && tErr.After(t0) {
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

const (
	propNS            = "de.digineo.zackup:"
	propLastStart     = propNS + "last_start" // unix timestamp
	propLastSuccess   = propNS + "s_date"     // unix timestamp
	propLastSDuration = propNS + "s_duration" // duration
	propLastFailure   = propNS + "f_date"     // unix timestamp
	propLastFDuration = propNS + "f_duration" // duration
)

var zackupProps = strings.Join([]string{
	propLastStart,
	propLastSuccess,
	propLastSDuration,
	propLastFailure,
	propLastFDuration,
}, ",")

// LoadState reads the performance metrics stored in the data
func LoadState() error {
	return state.load()
}

// ExportState dumps the current performance metrics.
func ExportState() []HostMetrics {
	return state.export()
}

func (s *State) load() error {
	args := []string{
		"get", "-r", "-H", "-p",
		"-t", "filesystem",
		"-s", "local",
		"-o", "name,property,value",
		zackupProps,
		RootDataset,
	}

	o, e, err := exec("zfs", args...)

	logerr := func(err error) {
		f := appendStdlogs(logrus.Fields{
			logrus.ErrorKey: err,
			"command":       append([]string{"zfs"}, args...),
		}, o, e)
		log.WithFields(f).Error("failed to load state")
	}

	if err != nil {
		logerr(err)
		return err
	}

	isTab := func(r rune) bool { return r == '\t' }
	rds := RootDataset + "/"
	scan := bufio.NewScanner(o)

	s.mu.Lock()
	defer s.mu.Unlock()

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

		if cols[0] == RootDataset || !strings.HasPrefix(cols[0], rds) {
			l.Trace("ignore non-child dataset")
			continue // we're only interested in the child datasets
		}
		if !strings.HasPrefix(cols[1], propNS) {
			l.Trace("ignore non-zackup properties")
			continue // we're only interested in our properties
		}
		if cols[2] == "-" {
			l.Trace("ignore empty values")
			continue // ignore "unknown value"
		}

		host := strings.TrimPrefix(cols[0], rds)
		met, ok := s.results[host]
		if !ok {
			met = &metrics{}
			s.results[host] = met
		}

		ival, err := strconv.ParseInt(cols[2], 10, 64)
		if err != nil {
			l.Trace("ignore non-integer values")
			continue // ignore non-integer values
		}
		l.Trace("accepting line")

		switch cols[1] {
		case propLastStart:
			met.StartedAt = time.Unix(ival, 0)
		case propLastSuccess:
			t := time.Unix(ival, 0)
			met.SucceededAt = &t
		case propLastSDuration:
			met.SuccessDuration = time.Duration(ival) * time.Millisecond
		case propLastFailure:
			t := time.Unix(ival, 0)
			met.FailedAt = &t
		case propLastFDuration:
			met.FailureDuration = time.Duration(ival) * time.Millisecond
		default:
			// ignore
		}
	}

	if err := scan.Err(); err != nil {
		logerr(err)
		return err
	}
	return nil
}

func storeStart(host string, t time.Time) error {
	args := []string{
		"set",
		fmt.Sprintf("%s=%d", propLastStart, t.Unix()),
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
	var propTime, propDur = propLastFailure, propLastFDuration
	if success {
		propTime, propDur = propLastSuccess, propLastSDuration
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
				StartedAt:       met.StartedAt,
				SucceededAt:     met.SucceededAt,
				SuccessDuration: met.SuccessDuration,
				FailedAt:        met.FailedAt,
				FailureDuration: met.FailureDuration,
			},
		})
	}

	sort.Slice(ex, func(i, j int) bool {
		return ex[i].Host < ex[j].Host
	})

	s.mu.RUnlock()
	return
}
