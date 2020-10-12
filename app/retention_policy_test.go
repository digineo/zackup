package app

import (
	"testing"
	"time"

	"github.com/digineo/zackup/config"
	"github.com/stretchr/testify/assert"
)

func TestBucket_matches(t *testing.T) {
	t.Parallel()
	now := time.Now()

	for _, tc := range []struct {
		name string
		dur  time.Duration
	}{
		{"seconds", 10 * time.Second},
		{"day", daily},
		{"fortnight", 14 * daily},
		{"month", monthly},
		{"decade", 10 * yearly},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			subject := bucket{now.UnixNano(), int64(tc.dur)}

			assert.False(t, subject.matches(now.Add(-time.Nanosecond)))
			assert.True(t, subject.matches(now))
			assert.True(t, subject.matches(now.Add(time.Nanosecond)))
			assert.True(t, subject.matches(now.Add(tc.dur-time.Nanosecond)))
			assert.False(t, subject.matches(now.Add(tc.dur)))
		})
	}

	t.Run("infinite", func(t *testing.T) {
		t.Parallel()
		subject := bucket{now.UnixNano(), -1}

		assert.True(t, subject.matches(now))
		assert.True(t, subject.matches(now.Add(100*time.Hour)))
		assert.True(t, subject.matches(now.Add(-100*time.Hour)))
	})
}

// rpConfig generates test cases.
type rpConfig struct {
	d, w, m, y *int
	omitTerm   bool
}

func (rp *rpConfig) D(n int) *rpConfig { rp.d = &n; return rp }
func (rp *rpConfig) M(n int) *rpConfig { rp.m = &n; return rp }
func (rp *rpConfig) W(n int) *rpConfig { rp.w = &n; return rp }
func (rp *rpConfig) Y(n int) *rpConfig { rp.y = &n; rp.omitTerm = true; return rp }

func (rp *rpConfig) newRetentionPolicy(now time.Time) retentionPolicy {
	return newRetentionPolicy(now, &config.RetentionConfig{
		Daily:   rp.d,
		Weekly:  rp.w,
		Monthly: rp.m,
		Yearly:  rp.y,
	})
}

func (rp *rpConfig) expectation(now time.Time) (pol retentionPolicy) {
	gen := func(n int, dur time.Duration) (list []bucket) {
		for i := 0; i < n; i++ {
			list = append(list, bucket{
				start:    now.Add(time.Duration(i) * dur).UnixNano(),
				duration: int64(dur),
			})
		}
		return list
	}

	if rp.d != nil {
		pol = append(pol, gen(*rp.d, daily)...)
	}
	if rp.w != nil {
		pol = append(pol, gen(*rp.w, weekly)...)
	}
	if rp.m != nil {
		pol = append(pol, gen(*rp.m, monthly)...)
	}
	if rp.y != nil {
		pol = append(pol, gen(*rp.y, yearly)...)
	}
	if !rp.omitTerm {
		pol = append(pol, bucket{now.UnixNano(), -1})
	}
	return pol
}

func TestNewRetentionPolicy(t *testing.T) {
	t.Parallel()
	const t0 = 1600000000_000000000 // 2020-09-13T12:26:40Z, arbitrarily chosen
	now := time.Unix(0, t0)

	t.Run("no config", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() { newRetentionPolicy(now, nil) })
	})

	tests := map[string]*rpConfig{
		"empty config": (&rpConfig{}),
		"0d":           (&rpConfig{}).D(0),                // same as empty config
		"0d 0w 0m":     (&rpConfig{}).D(0).W(0).M(0),      // same as empty config
		"0d 0w 0m 0y":  (&rpConfig{}).D(0).W(0).M(0).Y(0), // almost same as empty config
		"1d":           (&rpConfig{}).D(1),                // single day
		"3d":           (&rpConfig{}).D(3),                // multiple days
		"42w":          (&rpConfig{}).W(42),               // skipping over daily
		"1y":           (&rpConfig{}).Y(1),
		"2d 3w 4m 5y":  (&rpConfig{}).D(2).W(3).M(4).Y(5), // all fields
		"2d 3m":        (&rpConfig{}).D(2).M(3),           // skipping over weekly
		"2d 0w 3m":     (&rpConfig{}).D(2).W(0).M(3),      // disabling weekly, same as skipping over
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			expcect := tc.expectation(now)
			actual := tc.newRetentionPolicy(now)
			assert.EqualValues(t, expcect, actual)
		})
	}
}
