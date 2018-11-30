package app

import (
	"strconv"
	"strings"
	"time"
)

// system properties
const (
	propUsed            = "used"            // space used by dataset and its children (snapshots)
	propUsedBySnapshots = "usedbysnapshots" // space used by snapshots
	propCompressRatio   = "compressratio"   // compression achieved for the "used" space
)

// user properties (need a namespace)
const (
	propZackupNS                  = "de.digineo.zackup:"
	propZackupLastStart           = propZackupNS + "last_start" // unix timestamp
	propZackupLastSuccessDate     = propZackupNS + "s_date"     // unix timestamp
	propZackupLastSuccessDuration = propZackupNS + "s_duration" // duration
	propZackupLastFailureDate     = propZackupNS + "f_date"     // unix timestamp
	propZackupLastFailureDuration = propZackupNS + "f_duration" // duration
)

var zackupProps = strings.Join([]string{
	// system properties
	propUsed, propUsedBySnapshots, propCompressRatio,

	// user properties
	propZackupLastStart,
	propZackupLastSuccessDate, propZackupLastSuccessDuration,
	propZackupLastFailureDate, propZackupLastFailureDuration,
}, ",")

var propDecoder = map[string]func(*metrics, string) error{
	propUsed: func(m *metrics, value string) error {
		uval, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			m.SpaceUsedTotal = uval
		}
		return err
	},

	propUsedBySnapshots: func(m *metrics, value string) error {
		uval, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			m.SpaceUsedBySnapshots = uval
		}
		return err
	},

	propCompressRatio: func(m *metrics, value string) error {
		fval, err := strconv.ParseFloat(strings.TrimSuffix(value, "x"), 64)
		if err == nil {
			m.CompressionFactor = fval
		}
		return err
	},

	propZackupLastStart: func(m *metrics, value string) error {
		ival, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			m.StartedAt = time.Unix(ival, 0)
		}
		return err
	},

	propZackupLastSuccessDate: func(m *metrics, value string) error {
		ival, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			t := time.Unix(ival, 0)
			m.SucceededAt = &t
		}
		return err
	},

	propZackupLastSuccessDuration: func(m *metrics, value string) error {
		ival, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			m.SuccessDuration = time.Duration(ival) * time.Millisecond
		}
		return err
	},

	propZackupLastFailureDate: func(m *metrics, value string) error {
		ival, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			t := time.Unix(ival, 0)
			m.FailedAt = &t
		}
		return err
	},

	propZackupLastFailureDuration: func(m *metrics, value string) error {
		ival, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			m.FailureDuration = time.Duration(ival) * time.Millisecond
		}
		return err
	},
}
