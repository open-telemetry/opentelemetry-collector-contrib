// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package timeutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"

import (
	"fmt"
	"strings"
	"time"

	strptime "github.com/observiq/ctimefmt"
)

func StrptimeToGo(layout string) (string, error) {
	return strptime.ToNative(layout)
}

func GetLocation(location *string, layout *string) (*time.Location, error) {
	if location != nil && *location != "" {
		// If "location" is specified, it must be in the local timezone database
		loc, err := time.LoadLocation(*location)
		if err != nil {
			return nil, fmt.Errorf("failed to load location %s: %w", *location, err)
		}
		return loc, nil
	}

	if layout != nil && strings.HasSuffix(*layout, "Z") {
		// If a timestamp ends with 'Z', it should be interpretted at Zulu (UTC) time
		return time.UTC, nil
	}

	return time.Local, nil
}

func ParseGoTime(layout string, value any, location *time.Location) (time.Time, error) {
	timeValue, err := parseGotime(layout, value, location)
	if err != nil {
		return time.Time{}, err
	}
	return SetTimestampYear(timeValue), nil
}

func parseGotime(layout string, value interface{}, location *time.Location) (time.Time, error) {
	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return time.Time{}, fmt.Errorf("type %T cannot be parsed as a time", value)
	}

	result, err := time.ParseInLocation(layout, str, location)

	// Depending on the timezone database, we may get a pseudo-matching timezone
	// This is apparent when the zone is not "UTC", but the offset is still 0
	zone, offset := result.Zone()
	if offset != 0 || zone == "UTC" {
		return result, err
	}

	// Manually look up the location based on the zone
	loc, locErr := time.LoadLocation(zone)
	if locErr != nil {
		// can't correct offset, just return what we have
		return result, fmt.Errorf("failed to load location %s: %w", zone, locErr)
	}

	// Reparse the timestamp, with the location
	resultLoc, locErr := time.ParseInLocation(layout, str, loc)
	if locErr != nil {
		// can't correct offset, just return original result
		return result, err
	}

	return resultLoc, locErr
}

// SetTimestampYear sets the year of a timestamp to the current year.
// This is needed because year is missing from some time formats, such as rfc3164.
func SetTimestampYear(t time.Time) time.Time {
	if t.Year() > 0 {
		return t
	}
	n := Now()
	d := time.Date(n.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	// If the timestamp would be more than 7 days in the future using this year,
	// assume it's from last year.
	if d.After(n.AddDate(0, 0, 7)) {
		d = d.AddDate(-1, 0, 0)
	}
	return d
}

// Allows tests to override with deterministic value
var Now = time.Now
