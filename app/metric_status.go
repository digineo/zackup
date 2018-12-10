package app

import "fmt"

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
