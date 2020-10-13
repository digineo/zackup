package app

import (
	"testing"
	"time"
)

func TestMetricsStatus(t *testing.T) {
	t.Parallel()

	var t0 time.Time
	t1 := time.Date(2018, time.December, 9, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t2.Add(time.Hour)

	for expected, tt := range map[MetricStatus][]struct {
		t0   time.Time
		tOK  *time.Time
		tErr *time.Time
	}{
		StatusPrimed: {
			{t0, nil, nil}, // t0 == time.Zero
			{t0, &t1, &t1}, // && tOK == tErr
			{t0, &t1, &t2}, // && tErr > tOK
			{t0, &t3, &t2}, // && tOK > tErr
		},
		StatusUnknown: {
			{t1, &t1, &t1}, // t0 == tOK == tErr
			{t1, &t2, &t2}, // tOK > t0 && tErr > t0 && tOK == tErr
		},
		StatusRunning: {
			{t1, nil, nil}, // t0 > time.Zero
			{t2, &t1, nil}, // t0 > tOK
			{t2, nil, &t1}, // t0 > tErr
			{t2, &t1, &t1}, // t0 > tOK && t0 > tErr
		},
		StatusFailed: {
			{t1, nil, &t2}, // tErr > t0
			{t1, &t1, &t2}, // tErr > t0 && tErr > tOK
			{t1, &t2, &t3}, // tErr > tOK && tOK > t0 && tOK > tStart
			{t2, &t1, &t2}, // tErr >= t0 && tErr > tOK
		},
		StatusSuccess: {
			{t1, &t2, nil}, // tOK > t0
			{t1, &t2, &t1}, // tOK > t0 && tOK > tErr
			{t1, &t3, &t2}, // tOK > tErr && tErr > t0 && tErr > tStart
			{t2, &t2, &t1}, // tOK >= t0 && tOK > tErr
		},
	} {
		for i, tc := range tt {
			subject := metrics{
				StartedAt:   tc.t0,
				SucceededAt: tc.tOK,
				FailedAt:    tc.tErr,
			}
			actual := subject.Status()
			if actual != expected {
				t.Errorf("case %d: expected %s, got %s\n", i, expected, actual)
			}
		}
	}
}
