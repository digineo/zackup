package app

import (
	"time"

	"github.com/digineo/zackup/config"
)

// predefined bucket interval lengths.
const (
	daily   = 24 * time.Hour
	weekly  = 7 * daily
	monthly = 30 * daily
	yearly  = 360 * daily
)

// A bucket defines a time interval. See retentionPolicy for more notes.
type bucket struct {
	start    int64 // start of bucket interval (unix timestamp)
	duration int64 // length of interval, values <= 0 mean "infinite"
}

func (b *bucket) matches(st time.Time) bool {
	s64 := st.UnixNano()
	return b.duration <= 0 || (b.start <= s64 && s64 < b.start+b.duration)
}

// A retentionPolicy is an ordered list of buckets, which define a
// keep-or-delete policy for a set of snapshots.
//
// This list is assumed to be ordered first by the bucket's start time
// and then by its interval length, both in ascending order.
//
// Note: If one bucket has infinite length, it prematurely terminates
// the list evaluation.
type retentionPolicy []bucket

// allNil returns false if any element of list is not nil.
// The empty list will hence return true as well.
func allNil(list ...*int) bool {
	for _, i := range list {
		if i != nil {
			return false
		}
	}
	return true
}

func newRetentionPolicy(now time.Time, c *config.RetentionConfig) (rp retentionPolicy) {
	// We might need to a terminating element to the return value, otherwise all
	// snapshots older the last bucket item would be deleted.
	var addTerminator bool

	appendBuckets := func(dur time.Duration, count *int, rest ...*int) {
		if count == nil {
			if !addTerminator && allNil(rest...) {
				addTerminator = true
			}
			return
		}
		for n := 0; n < *count; n++ {
			rp = append(rp, bucket{
				now.Add(time.Duration(n) * dur).UnixNano(),
				int64(dur),
			})
		}
	}

	// keep sorted!
	appendBuckets(daily, c.Daily, c.Weekly, c.Monthly, c.Yearly)
	appendBuckets(weekly, c.Weekly, c.Monthly, c.Yearly)
	appendBuckets(monthly, c.Monthly, c.Yearly)
	appendBuckets(yearly, c.Yearly)

	if addTerminator {
		rp = append(rp, bucket{now.UnixNano(), -1})
	}

	return rp
}

// apply partitions the given input snapshot list into two new slices: one
// containing snapshots to keep and one containing snapshots to delete.
// You need to pass a reference time for the policy's buckets to anchor on.
func (rp retentionPolicy) apply(snapshots []snapshot) (toKeep, toDelete []snapshot) {
	// iterate over rp and snapshots and sort snapshots[i] in either toKeep or
	// toDelete, depending on weather snapshots[i].Time < now.Add(rp[j].interval)

	mark := make(map[int]bool)
	matchAny := make(map[int]bool)

	for _, b := range rp {
		curr := -1 // bucket content (index into snapshots)

		for i, s := range snapshots {
			if b.matches(s.Time) {
				matchAny[i] = true

				switch {
				case curr < 0:
					curr = i
				case s.Time.After(snapshots[curr].Time):
					mark[curr] = true
					curr = i
				default:
					mark[i] = true
				}
			}
		}

		if b.duration <= 0 {
			break // no need to continue
		}
	}

	for i, s := range snapshots {
		if mark[i] || !matchAny[i] {
			toDelete = append(toDelete, s)
		} else {
			toKeep = append(toKeep, s)
		}
	}

	return toKeep, toDelete
}
