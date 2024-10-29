// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
)

// StrptimeKey is literally "strptime", and is the default layout type
const StrptimeKey = "strptime"

// GotimeKey is literally "gotime" and uses Golang's native time.Parse
const GotimeKey = "gotime"

// EpochKey is literally "epoch" and can parse seconds and/or subseconds
const EpochKey = "epoch"

// NativeKey is literally "native" and refers to Golang's native time.Time
const NativeKey = "native" // provided for operator development

// NewTimeParser creates a new time parser with default values
func NewTimeParser() TimeParser {
	return TimeParser{
		LayoutType: StrptimeKey,
	}
}

// TimeParser is a helper that parses time onto an entry.
type TimeParser struct {
	ParseFrom  *entry.Field `mapstructure:"parse_from"`
	Layout     string       `mapstructure:"layout"`
	LayoutType string       `mapstructure:"layout_type"`
	Location   string       `mapstructure:"location"`

	location *time.Location
}

// Unmarshal starting from default settings
func (t *TimeParser) Unmarshal(component *confmap.Conf) error {
	err := component.Unmarshal(t, confmap.WithIgnoreUnused())
	if err != nil {
		return err
	}
	if t.LayoutType == "" {
		t.LayoutType = StrptimeKey
	}
	return nil
}

// IsZero returns true if the TimeParser is not a valid config
func (t *TimeParser) IsZero() bool {
	return t.Layout == ""
}

// Validate validates a TimeParser, and reconfigures it if necessary
func (t *TimeParser) Validate() error {
	if t.ParseFrom == nil {
		return fmt.Errorf("missing required parameter 'parse_from'")
	}

	if t.Layout == "" && t.LayoutType != "native" {
		return errors.NewError("missing required configuration parameter `layout`", "")
	}

	switch t.LayoutType {
	case NativeKey: // ok
	case GotimeKey:
		if err := timeutils.ValidateGotime(t.Layout); err != nil {
			return errors.Wrap(err, "invalid gotime layout")
		}
	case StrptimeKey:
		if err := timeutils.ValidateStrptime(t.Layout); err != nil {
			return errors.Wrap(err, "invalid strptime layout")
		}
		var err error
		t.Layout, err = timeutils.StrptimeToGotime(t.Layout)
		if err != nil {
			return errors.Wrap(err, "parse strptime layout")
		}
		t.LayoutType = GotimeKey
	case EpochKey:
		switch t.Layout {
		case "s", "ms", "us", "ns", "s.ms", "s.us", "s.ns": // ok
		default:
			return errors.NewError(
				"invalid `layout` for `epoch` type",
				"specify 's', 'ms', 'us', 'ns', 's.ms', 's.us', or 's.ns'",
			)
		}
	default:
		return errors.NewError(
			fmt.Sprintf("unsupported layout_type %s", t.LayoutType),
			"valid values are 'strptime', 'gotime', and 'epoch'",
		)
	}

	if t.LayoutType == GotimeKey { // also covers StrptimeKey because it was remapped above
		if err := t.setLocation(); err != nil {
			return errors.Wrap(err, "invalid 'location'")
		}
	}

	return nil
}

func (t *TimeParser) setLocation() error {
	if t.Location != "" {
		// If "location" is specified, it must be in the local timezone database
		loc, err := time.LoadLocation(t.Location)
		if err != nil {
			return fmt.Errorf("failed to load location %s: %w", t.Location, err)
		}
		t.location = loc
		return nil
	}

	if strings.HasSuffix(t.Layout, "Z") {
		// If a timestamp ends with 'Z', it should be interpretted at Zulu (UTC) time
		t.location = time.UTC
		return nil
	}

	t.location = time.Local
	return nil
}

// Parse will parse time from a field and attach it to the entry
func (t *TimeParser) Parse(entry *entry.Entry) error {
	value, ok := entry.Get(t.ParseFrom)
	if !ok {
		return errors.NewError(
			"log entry does not have the expected parse_from field",
			"ensure that all entries forwarded to this parser contain the parse_from field",
			"parse_from", t.ParseFrom.String(),
		)
	}

	switch t.LayoutType {
	case NativeKey:
		timeValue, ok := value.(time.Time)
		if !ok {
			return fmt.Errorf("native time.Time field required, but found %v of type %T", value, value)
		}
		entry.Timestamp = timeutils.SetTimestampYear(timeValue)
	case GotimeKey:
		timeValue, err := timeutils.ParseGotime(t.Layout, value, t.location)
		if err != nil {
			return err
		}
		// timeutils.ParseGotime calls timeutils.SetTimestampYear before returning the timeValue
		entry.Timestamp = timeValue
	case EpochKey:
		timeValue, err := t.parseEpochTime(value)
		if err != nil {
			return err
		}
		entry.Timestamp = timeutils.SetTimestampYear(timeValue)
	default:
		return fmt.Errorf("unsupported layout type: %s", t.LayoutType)
	}

	return nil
}

func (t *TimeParser) parseEpochTime(value any) (time.Time, error) {
	stamp, err := getEpochStamp(t.Layout, value)
	if err != nil {
		return time.Time{}, err
	}

	switch t.Layout {
	case "s", "ms", "us", "ns":
		i, err := strconv.ParseInt(stamp, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid value '%v' for layout '%s'", stamp, t.Layout)
		}
		return toTime[t.Layout](i), nil
	case "s.ms", "s.us", "s.ns":
		secSubsec := strings.Split(stamp, ".")
		if len(secSubsec) != 2 {
			return time.Time{}, fmt.Errorf("invalid value '%v' for layout '%s'", stamp, t.Layout)
		}
		sec, secErr := strconv.ParseInt(secSubsec[0], 10, 64)
		subsec, subsecErr := strconv.ParseInt(secSubsec[1], 10, 64)
		if secErr != nil || subsecErr != nil {
			return time.Time{}, fmt.Errorf("invalid value '%v' for layout '%s'", stamp, t.Layout)
		}
		return time.Unix(sec, subsec*subsecToNs[t.Layout]), nil
	default:
		return time.Time{}, fmt.Errorf("invalid layout '%s'", t.Layout)
	}
}

func getEpochStamp(layout string, value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case int, int32, int64, uint32, uint64:
		switch layout {
		case "s", "ms", "us", "ns":
			return fmt.Sprintf("%d", v), nil
		case "s.ms", "s.us", "s.ns":
			return fmt.Sprintf("%d.0", v), nil
		default:
			return "", fmt.Errorf("invalid layout '%s'", layout)
		}
	case float64:
		switch layout {
		case "s", "ms", "us", "ns":
			return fmt.Sprintf("%d", int64(v)), nil
		case "s.ms":
			return fmt.Sprintf("%10.3f", v), nil
		case "s.us":
			return fmt.Sprintf("%10.6f", v), nil
		case "s.ns":
			return fmt.Sprintf("%10.9f", v), nil
		default:
			return "", fmt.Errorf("invalid layout '%s'", layout)
		}
	default:
		return "", fmt.Errorf("type %T cannot be parsed as a time", v)
	}
}

type toTimeFunc = func(int64) time.Time

var toTime = map[string]toTimeFunc{
	"s":  func(s int64) time.Time { return time.Unix(s, 0) },
	"ms": func(ms int64) time.Time { return time.Unix(ms/1e3, (ms%1e3)*1e6) },
	"us": func(us int64) time.Time { return time.Unix(us/1e6, (us%1e6)*1e3) },
	"ns": func(ns int64) time.Time { return time.Unix(0, ns) },
}
var subsecToNs = map[string]int64{"s.ms": 1e6, "s.us": 1e3, "s.ns": 1}
