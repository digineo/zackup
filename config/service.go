package config

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// ServiceConfig represents application-level config options.
type ServiceConfig struct {
	Parallel    uint8  `yaml:"parallel"`
	RootDataset string `yaml:"root_dataset"`
	MountBase   string `yaml:"mount_base"`
	LogLevel    string `yaml:"log_level"`

	RSyncPath string `yaml:"rsync_bin"`
	SSHPath   string `yaml:"ssh_bin"`

	Daemon struct {
		Schedule schedule `yaml:"schedule"`
		Jitter   duration `yaml:"jitter"`
	} `yaml:"daemon"`
}

type duration time.Duration

func (d *duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*d = duration(dur)
	return nil
}

type schedule struct {
	h, m, s int // hour, minute and second values
}

var (
	errInvalidScheduleFormat = errors.New("invalid format, expected HH:MM:SS")
	errScheduleOutOfRange    = errors.New("out of range, must be between 00:00:00 and 23:59:59")
)

func (sc *schedule) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	f := strings.SplitN(s, ":", 4)
	if len(f) != 3 {
		return errInvalidScheduleFormat
	}

	i := make([]int, 0, 3)
	for pos := range f {
		val, err := strconv.Atoi(f[pos])
		if err != nil {
			return err
		}
		if val < 0 || (pos == 0 && val > 23) || (pos > 0 && val > 59) {
			return errScheduleOutOfRange
		}
		i = append(i, val)
	}

	*sc = schedule{i[0], i[1], i[2]}
	return nil
}

// Next returns the next schedule time > t.
func (sc *schedule) Next(t *time.Time) time.Time {
	y, mon, d := t.Date()
	h, m, s := t.Clock()

	// next day if Time.now >= Time.now.adjust(hour: sc.h, minute: sc.m, second: sc.s)
	if h > sc.h || h == sc.h && m > sc.m || h == sc.h && m == sc.m && s >= sc.s {
		d++
	}
	return time.Date(y, mon, d, sc.h, sc.m, sc.s, 0, t.Location())
}

func (sc *schedule) String() string {
	return fmt.Sprintf("%02d:%02d:%02d", sc.h, sc.m, sc.s)
}

// NextSchedule returns the next schedule time. It applies the configured
// jitter, so the result is not be stable (i.e. neither is it monotonically
// decreasing for increasing reference times, nor returns it the same duration
// for the same ref).
func (s *ServiceConfig) NextSchedule(ref time.Time) time.Time {
	utc := ref.UTC()

	// advance ref time, so that we don't end up in the range [utc, utc+jit/2)
	if jit := time.Duration(s.Daemon.Jitter / 2); jit > 0 {
		utc = utc.Add(jit)
	}
	next := s.Daemon.Schedule.Next(&utc)

	// apply jitter
	if jit := int64(s.Daemon.Jitter); jit > 0 {
		rnd := rand.Int63n(jit) - jit/2
		next = next.Add(time.Duration(rnd).Truncate(100 * time.Millisecond))
	}

	return next
}
