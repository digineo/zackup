package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// system properties.
const (
	propUsedBySnapshots      = "usedbysnapshots"      // space used by snapshots
	propUsedByDataset        = "usedbydataset"        // space used by dataset itself
	propUsedByChildren       = "usedbychildren"       // space used by children of dataset
	propUsedByRefReservation = "usedbyrefreservation" // reserved space
	propCompressRatio        = "compressratio"        // compression achieved for the "used" space
)

// user properties (need a namespace).
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
	propUsedBySnapshots,
	propUsedByDataset,
	propUsedByChildren,
	propUsedByRefReservation,
	propCompressRatio,

	// user properties
	propZackupLastStart,
	propZackupLastSuccessDate, propZackupLastSuccessDuration,
	propZackupLastFailureDate, propZackupLastFailureDuration,
}, ",")

type decodeError struct {
	prop  string
	cause error
}

func (err *decodeError) Error() string {
	return fmt.Sprintf("failed to decode zprop %s: %v", err.prop, err.cause)
}

func (err *decodeError) Unwrap() error {
	return err.cause
}

var propDecoder = map[string]func(*metrics, string) error{
	propUsedBySnapshots: func(m *metrics, value string) error {
		uval, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			m.SpaceUsedBySnapshots = uval
		}
		return &decodeError{propUsedBySnapshots, err}
	},

	propUsedByDataset: func(m *metrics, value string) error {
		uval, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			m.SpaceUsedByDataset = uval
		}
		return &decodeError{propUsedByDataset, err}
	},

	propUsedByChildren: func(m *metrics, value string) error {
		uval, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			m.SpaceUsedByChildren = uval
		}
		return &decodeError{propUsedByChildren, err}
	},

	propUsedByRefReservation: func(m *metrics, value string) error {
		uval, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			m.SpaceUsedByRefReservation = uval
		}
		return &decodeError{propUsedByRefReservation, err}
	},

	propCompressRatio: func(m *metrics, value string) error {
		fval, err := strconv.ParseFloat(strings.TrimSuffix(value, "x"), 64)
		if err == nil {
			m.CompressionFactor = fval
		}
		return &decodeError{propCompressRatio, err}
	},

	propZackupLastStart: func(m *metrics, value string) error {
		ival, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			m.StartedAt = time.Unix(ival, 0)
		}
		return &decodeError{propZackupLastStart, err}
	},

	propZackupLastSuccessDate: func(m *metrics, value string) error {
		ival, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			t := time.Unix(ival, 0)
			m.SucceededAt = &t
		}
		return &decodeError{propZackupLastSuccessDate, err}
	},

	propZackupLastSuccessDuration: func(m *metrics, value string) error {
		ival, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			m.SuccessDuration = time.Duration(ival) * time.Millisecond
		}
		return &decodeError{propZackupLastSuccessDuration, err}
	},

	propZackupLastFailureDate: func(m *metrics, value string) error {
		ival, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			t := time.Unix(ival, 0)
			m.FailedAt = &t
		}
		return &decodeError{propZackupLastFailureDate, err}
	},

	propZackupLastFailureDuration: func(m *metrics, value string) error {
		ival, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			m.FailureDuration = time.Duration(ival) * time.Millisecond
		}
		return &decodeError{propZackupLastFailureDuration, err}
	},
}
