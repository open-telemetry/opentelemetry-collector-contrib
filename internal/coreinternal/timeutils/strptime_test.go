package timeutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// These test cases are in a variable so they can be called from TestTimeParserStrptimeCgo in strptime_cgo_test.go
var strptimeTests = []struct {
	name     string
	expected time.Time
	format   string
	// samples is a list of strings that are all expected to parse to the same time
	samples  []string
	location string
}{
	{
		name:     "unix-utc",
		expected: time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC),
		format:   "%a %b %e %H:%M:%S %Z %Y",
		samples:  []string{"Mon Jan 2 15:04:05 UTC 2006"},
	},
	{
		// https://github.com/bminor/glibc/blob/04e750e75b73957cf1c791535a3f4319534a52fc/time/strptime_l.c#L778-L816
		name:     "iso-8601-hst",
		expected: time.Date(2007, time.April, 5, 6, 7, 8, 0, time.FixedZone("HST", -10*3600)),
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
		expected: time.Date(2007, time.April, 5, 6, 7, 8, 0, time.FixedZone("HST", -10*3600)),
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
}

func TestParseStrptime(t *testing.T) {
	// Mountain Standard Time
	mst, err := time.LoadLocation("MST")
	require.NoError(t, err)

	for _, tt := range strptimeTests {
		t.Run(tt.name, func(t *testing.T) {
			for _, s := range tt.samples {
				t.Run(s, func(t *testing.T) {
					got, err := ParseStrptime(tt.format, s, mst)
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
