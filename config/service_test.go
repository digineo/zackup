package config

import (
	"math/rand"
	"testing"
	"time"
)

func TestNextSchedule(t *testing.T) {
	svc := &ServiceConfig{}
	svc.Daemon.Schedule = schedule{4, 0, 0}   // 04:00:00
	svc.Daemon.Jitter = duration(time.Second) // 03:59:59.5 - 04:00:00.5

	rand.Seed(0)
	// ref = (any) date, but 31 seconds before designated schedule
	ref := time.Date(2018, time.December, 9, 3, 59, 29, 0, time.UTC)

	// after i = 0, ref = 2018-12-09 03:59:34
	//       i = 1, ref = 2018-12-09 03:59:39
	//       ...
	//       i = 5, ref = 2018-12-09 03:59:59
	for i := 0; i < 6; i++ {
		ref = ref.Add(5 * time.Second)

		next := svc.NextSchedule(ref)
		if next.Day() != ref.Day() {
			t.Errorf("a%d ref=%s, unexpected jump to %s\n",
				i, ref.Format(time.RFC3339), next.Format(time.RFC3339))
		}
	}

	// after i = 0, ref = 2018-12-09 04:00:04
	// 	     i = 1, ref = 2018-12-09 04:00:09
	//       ...
	//       i = 5, ref = 2018-12-09 04:00:29
	for i := 0; i < 6; i++ {
		ref = ref.Add(5 * time.Second)

		next := svc.NextSchedule(ref)
		if next.Day() != ref.Day()+1 {
			t.Errorf("b%d ref=%s, unexpected jump to %s\n",
				i, ref.Format(time.RFC3339), next.Format(time.RFC3339))
		}
	}
}
