package app

import (
	"sync"
	"time"
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
	if s, ok := s.results[host]; ok {
		s.SucceededAt = &t
		s.SuccessDuration = t.Sub(s.StartedAt)
	}
	s.mu.Unlock()
}

func (s *State) failure(host string) {
	t := time.Now().UTC()
	s.mu.Lock()
	if s, ok := s.results[host]; ok {
		s.FailedAt = &t
		s.FailureDuration = t.Sub(s.StartedAt)
	}
	s.mu.Unlock()
}
