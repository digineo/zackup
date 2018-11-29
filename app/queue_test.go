package app

import (
	"fmt"
	"testing"
	"time"
)

func TestLastSeenEntry(t *testing.T) {
	t1 := time.Date(2018, time.December, 9, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(duplicateDetectionTime - time.Millisecond)
	t3 := t1.Add(duplicateDetectionTime)
	t4 := t1.Add(duplicateDetectionTime + time.Millisecond)

	ft := func(t time.Time) string { return t.Format("15:04:05.000") }

	for name, tt := range map[string][]struct {
		s, e     time.Time
		ref      *time.Time
		expected bool
	}{
		"s=e=0": { // did not start yet
			{ref: &t1, expected: false},
			{ref: &t2, expected: false},
			{ref: &t3, expected: false},
			{ref: &t4, expected: false},
		},
		"s=t1, e=0": { // did not finish yet
			{ref: &t1, expected: true, s: t1},
			{ref: &t2, expected: true, s: t1},
			{ref: &t3, expected: true, s: t1},
			{ref: &t4, expected: true, s: t1},
		},
		"s=e=t1": { // finished at t1, did not start again
			{ref: &t1, expected: true, s: t1, e: t1},
			{ref: &t2, expected: true, s: t1, e: t1},
			{ref: &t3, expected: true, s: t1, e: t1},  // t3 = e+ttl
			{ref: &t4, expected: false, s: t1, e: t1}, // t4 > e+ttl
		},
		"s=t1, e=t2": { // similar to before
			{ref: &t1, expected: true, s: t1, e: t2},
			{ref: &t2, expected: true, s: t1, e: t2},
			{ref: &t3, expected: true, s: t1, e: t2},
			{ref: &t4, expected: true, s: t1, e: t2},
		},
		"s=t2, e=t1": { // finished at t1, but startd again
			{ref: &t1, expected: true, s: t2, e: t1},
			{ref: &t2, expected: true, s: t2, e: t1},
			{ref: &t3, expected: true, s: t2, e: t1},
			{ref: &t4, expected: true, s: t2, e: t1},
		},
	} {
		for i, tc := range tt {
			t.Run(fmt.Sprintf("%s/%d", name, i), func(t *testing.T) {
				ent := &lastSeenEntry{start: tc.s, finish: tc.e}
				actual := ent.recent(tc.ref)

				if actual != tc.expected {
					t.Fatalf("expected ent{%s,%s}.recent(%s) to be %v\n",
						ft(tc.s), ft(tc.e), ft(*tc.ref), tc.expected)
				}
			})
		}
	}
}
