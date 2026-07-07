// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package timeutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mustLoadLocation(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

var (
	// Hawaii Standard Time
	hst = mustLoadLocation("HST")
	// Mountain Standard Time
	mst = mustLoadLocation("MST")
)

// These test cases are in a variable so they can be called from TestTimeParserStrptimeCgo in strptime_cgo_test.go
var strptimeTests = []struct {
	name     string
	expected time.Time
	format   string
	// samples is a list of strings that are all expected to parse to the same time
	samples []string
}{
	{
		name:     "unix-utc",
		expected: time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC),
		format:   "%a %b %e %H:%M:%S %z %Y",
		samples: []string{
			"Mon Jan 2 15:04:05 Z 2006",
			"Mon Jan 2 15:4:5 Z 2006",
		},
	},
	{
		// https://github.com/bminor/glibc/blob/04e750e75b73957cf1c791535a3f4319534a52fc/time/strptime_l.c#L778-L816
		name:     "iso-8601-hst",
		expected: time.Date(2007, time.April, 5, 6, 7, 8, 0, hst),
		format:   "%Y-%m-%dT%H:%M:%S%z",
		samples: []string{
			"2007-04-05T06:07:08-10:00",
			"2007-04-05T06:07:08-1000",
			"2007-04-05T06:07:08-10",
			"2007-04-05T06:07:08  -10:00",
			"2007-04-05T06:07:08  -1000",
			"2007-4-5T6:7:8  -10",
			"2007-4-5T6:7:8-10:00",
			"2007-4-5T6:7:8-1000",
			"2007-4-5T6:7:8-10",
			"2007-4-5T6:7:8  -10:00",
			"2007-4-5T6:7:8  -1000",
			"2007-4-5T6:7:8  -10",
		},
	},
	{
		name:     "short-year-hst",
		expected: time.Date(2007, time.April, 5, 6, 7, 8, 0, hst),
		format:   "%y-%m-%dT%H:%M:%S%z",
		samples: []string{
			"07-04-05T06:07:08-10:00",
		},
	},
	{
		name:     "iso-8601-utc",
		expected: time.Date(2007, time.April, 5, 12, 30, 46, 0, time.UTC),
		format:   "%Y-%m-%dT%H:%M:%S%z",
		samples: []string{
			"2007-4-5T12:30:46+0000",
			"2007-04-05T12:30:46+00:00",
			"2007-04-05T12:30:46+0000",
			"2007-04-05T12:30:46+00",
			"2007-04-05T12:30:46Z",
			"2007-04-05T12:30:46  +00:00",
			"2007-04-05T12:30:46  +0000",
			"2007-04-05T12:30:46  +00",
			"2007-04-05T12:30:46  Z",
		},
	},
	{
		name:     "no-separators",
		expected: time.Date(2023, time.February, 3, 5, 6, 7, 0, mst),
		format:   "%Y%m%d%H%M%S",
		samples: []string{
			"20230203050607",
			"2023 02 03 05 06 07",
		},
	},
	{
		name:     "iso-8601-utc",
		expected: time.Date(2007, time.April, 5, 12, 30, 46, 0, time.UTC),
		format:   "%Y-%m-%dT%H:%M:%S%z",
		samples: []string{
			"2007-4-5T12:30:46+0000",
			"2007-04-05T12:30:46+00:00",
			"2007-04-05T12:30:46+0000",
			"2007-04-05T12:30:46+00",
			"2007-04-05T12:30:46Z",
			"2007-04-05T12:30:46  +00:00",
			"2007-04-05T12:30:46  +0000",
			"2007-04-05T12:30:46  +00",
			"2007-04-05T12:30:46  Z",
		},
	},
	{
		name:     "12h-time",
		expected: time.Date(2023, time.February, 3, 15, 6, 7, 0, mst),
		format:   "%Y-%m-%d %I:%M:%S %p",
		samples: []string{
			"2023-02-03 03:06:07 PM",
			"2023-02-03 3:6:7 PM",
		},
	},
	{
		name:     "macros-F-T",
		expected: time.Date(2007, time.April, 5, 12, 30, 46, 0, time.UTC),
		format:   "%FT%T%z",
		samples: []string{
			"2007-4-5T12:30:46+0000",
			"2007-04-05T12:30:46+00:00",
			"2007-04-05T12:30:46+0000",
			"2007-04-05T12:30:46+00",
			"2007-04-05T12:30:46Z",
			"2007-04-05T12:30:46  +00:00",
			"2007-04-05T12:30:46  +0000",
			"2007-04-05T12:30:46  +00",
			"2007-04-05T12:30:46  Z",
		},
	},
	{
		name:     "macro-R",
		expected: time.Date(2007, time.April, 5, 13, 30, 0, 0, mst),
		format:   "%R %Y-%m-%d",
		samples: []string{
			"13:30 2007-4-5",
			"13:30 2007-04-05",
		},
	},
	{
		name:     "macros-D-T",
		expected: time.Date(2007, time.April, 5, 12, 30, 46, 0, mst),
		format:   "%DT%T",
		samples: []string{
			"4/5/07T12:30:46",
			"04/05/07T12:30:46",
		},
	},
	{
		name:     "macros-x-T",
		expected: time.Date(2007, time.April, 5, 12, 30, 46, 0, mst),
		format:   "%xT%T",
		samples: []string{
			"4/5/07T12:30:46",
			"04/05/07T12:30:46",
		},
	},
	{
		name:     "macro-c",
		expected: time.Date(2007, time.April, 5, 13, 30, 46, 0, mst),
		format:   "%c",
		samples: []string{
			"Thu Apr 05 13:30:46 2007",
			"Thu Apr 5 13:30:46 2007",
		},
	},
}

func TestParseStrptime(t *testing.T) {
	for _, tt := range strptimeTests {
		t.Run(tt.name, func(t *testing.T) {
			for _, s := range tt.samples {
				t.Run(s, func(t *testing.T) {
					parser, err := NewStrptimeParser(tt.format)
					require.NoError(t, err)
					got, err := parser.Parse(s, mst)
					if err != nil {
						t.Logf("ParseError: %#v", err)
					}
					require.NoError(t, err)
					// Use WithinDuration instead of Equal so the timezone name is ignored.
					require.WithinDuration(t, tt.expected, got, 0)
				})
			}
		})
	}
}
