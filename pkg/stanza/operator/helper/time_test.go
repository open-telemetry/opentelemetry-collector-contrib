// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestIsZero(t *testing.T) {
	require.True(t, (&TimeParser{}).IsZero())
	require.False(t, (&TimeParser{Layout: "strptime"}).IsZero())
}

func TestTimeParser(t *testing.T) {
	// Mountain Standard Time
	mst, err := time.LoadLocation("MST")
	require.NoError(t, err)

	// Hawaiian Standard Time
	hst, err := time.LoadLocation("HST")
	require.NoError(t, err)

	// override with deterministic value
	timeutils.Now = func() time.Time { return time.Date(2020, 12, 16, 17, 0, 0, 0, mst) }

	testCases := []struct {
		name           string
		sample         interface{}
		expected       time.Time
		gotimeLayout   string
		strptimeLayout string
		location       string
	}{
		{
			name:           "unix-utc",
			sample:         "Mon Jan 2 15:04:05 UTC 2006",
			expected:       time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC),
			gotimeLayout:   "Mon Jan 2 15:04:05 MST 2006",
			strptimeLayout: "%a %b %e %H:%M:%S %Z %Y",
		},
		{
			name:           "unix-utc-ignore-location", // because timezone is included in timestamp
			sample:         "Mon Jan 2 15:04:05 UTC 2006",
			expected:       time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC),
			gotimeLayout:   "Mon Jan 2 15:04:05 MST 2006",
			strptimeLayout: "%a %b %e %H:%M:%S %Z %Y",
			location:       hst.String(),
		},
		{
			name:           "unix-mst",
			sample:         "Mon Jan 2 15:04:05 MST 2006",
			expected:       time.Date(2006, time.January, 2, 15, 4, 5, 0, mst),
			gotimeLayout:   "Mon Jan 2 15:04:05 MST 2006",
			strptimeLayout: "%a %b %e %H:%M:%S %Z %Y",
		},
		{
			name:           "unix-hst",
			sample:         "Mon Jan 2 15:04:05 HST 2006",
			expected:       time.Date(2006, time.January, 2, 15, 4, 5, 0, hst),
			gotimeLayout:   "Mon Jan 2 15:04:05 MST 2006",
			strptimeLayout: "%a %b %e %H:%M:%S %Z %Y",
		},
		{
			name:           "almost-unix",
			sample:         "Mon Jan 02 15:04:05 MST 2006",
			expected:       time.Date(2006, time.January, 2, 15, 4, 5, 0, mst),
			gotimeLayout:   "Mon Jan 02 15:04:05 MST 2006",
			strptimeLayout: "%a %b %d %H:%M:%S %Z %Y",
		},
		{
			name:           "kitchen",
			sample:         "12:34PM",
			expected:       time.Date(timeutils.Now().Year(), 1, 1, 12, 34, 0, 0, time.Local),
			gotimeLayout:   time.Kitchen,
			strptimeLayout: "%H:%M%p",
		},
		{
			name:           "kitchen-utc",
			sample:         "12:34PM",
			expected:       time.Date(timeutils.Now().Year(), 1, 1, 12, 34, 0, 0, time.UTC),
			gotimeLayout:   time.Kitchen,
			strptimeLayout: "%H:%M%p",
			location:       time.UTC.String(),
		},
		{
			name:           "kitchen-location",
			sample:         "12:34PM",
			expected:       time.Date(timeutils.Now().Year(), 1, 1, 12, 34, 0, 0, hst),
			gotimeLayout:   time.Kitchen,
			strptimeLayout: "%H:%M%p",
			location:       hst.String(),
		},
		{
			name:           "kitchen-bytes",
			sample:         []byte("12:34PM"),
			expected:       time.Date(timeutils.Now().Year(), 1, 1, 12, 34, 0, 0, time.Local),
			gotimeLayout:   time.Kitchen,
			strptimeLayout: "%H:%M%p",
		},
		{
			name:           "debian-syslog",
			sample:         "Jun 09 11:39:45",
			expected:       time.Date(timeutils.Now().Year(), time.June, 9, 11, 39, 45, 0, time.Local),
			gotimeLayout:   "Jan 02 15:04:05",
			strptimeLayout: "%b %d %H:%M:%S",
		},
		{
			name:           "debian-syslog-utc",
			sample:         "Jun 09 11:39:45",
			expected:       time.Date(timeutils.Now().Year(), time.June, 9, 11, 39, 45, 0, time.UTC),
			gotimeLayout:   "Jan 02 15:04:05",
			strptimeLayout: "%b %d %H:%M:%S",
			location:       time.UTC.String(),
		},
		{
			name:           "debian-syslog-location",
			sample:         "Jun 09 11:39:45",
			expected:       time.Date(timeutils.Now().Year(), time.June, 9, 11, 39, 45, 0, hst),
			gotimeLayout:   "Jan 02 15:04:05",
			strptimeLayout: "%b %d %H:%M:%S",
			location:       hst.String(),
		},
		{
			name:           "opendistro",
			sample:         "2020-06-09T15:39:58",
			expected:       time.Date(2020, time.June, 9, 15, 39, 58, 0, time.Local),
			gotimeLayout:   "2006-01-02T15:04:05",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S",
		},
		{
			name:           "opendistro-utc",
			sample:         "2020-06-09T15:39:58",
			expected:       time.Date(2020, time.June, 9, 15, 39, 58, 0, time.UTC),
			gotimeLayout:   "2006-01-02T15:04:05",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S",
			location:       time.UTC.String(),
		},
		{
			name:           "opendistro-location",
			sample:         "2020-06-09T15:39:58",
			expected:       time.Date(2020, time.June, 9, 15, 39, 58, 0, hst),
			gotimeLayout:   "2006-01-02T15:04:05",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S",
			location:       hst.String(),
		},
		{
			name:           "postgres",
			sample:         "2019-11-05 10:38:35.118 HST",
			expected:       time.Date(2019, time.November, 5, 10, 38, 35, 118*1000*1000, hst),
			gotimeLayout:   "2006-01-02 15:04:05.999 MST",
			strptimeLayout: "%Y-%m-%d %H:%M:%S.%L %Z",
		},
		{
			name:           "ibm-mq",
			sample:         "3/4/2018 11:52:29",
			expected:       time.Date(2018, time.March, 4, 11, 52, 29, 0, time.Local),
			gotimeLayout:   "1/2/2006 15:04:05",
			strptimeLayout: "%q/%g/%Y %H:%M:%S",
		},
		{
			name:           "ibm-mq-utc",
			sample:         "3/4/2018 11:52:29",
			expected:       time.Date(2018, time.March, 4, 11, 52, 29, 0, time.UTC),
			gotimeLayout:   "1/2/2006 15:04:05",
			strptimeLayout: "%q/%g/%Y %H:%M:%S",
			location:       time.UTC.String(),
		},
		{
			name:           "ibm-mq-location",
			sample:         "3/4/2018 11:52:29",
			expected:       time.Date(2018, time.March, 4, 11, 52, 29, 0, hst),
			gotimeLayout:   "1/2/2006 15:04:05",
			strptimeLayout: "%q/%g/%Y %H:%M:%S",
			location:       hst.String(),
		},
		{
			name:           "cassandra",
			sample:         "2019-11-27T09:34:32.901-1000",
			expected:       time.Date(2019, time.November, 27, 9, 34, 32, 901*1000*1000, hst),
			gotimeLayout:   "2006-01-02T15:04:05.999-0700",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%L%z",
		},
		{
			name:           "cassandra-ignore-location", // because timezone is included in timestamp
			sample:         "2019-11-27T09:34:32.901-1000",
			expected:       time.Date(2019, time.November, 27, 9, 34, 32, 901*1000*1000, hst),
			gotimeLayout:   "2006-01-02T15:04:05.999-0700",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%L%z",
			location:       hst.String(),
		},
		{
			name:           "oracle",
			sample:         "2019-10-15T10:42:01.900436-10:00",
			expected:       time.Date(2019, time.October, 15, 10, 42, 01, 900436*1000, hst),
			gotimeLayout:   "2006-01-02T15:04:05.999999-07:00",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%f%j",
		},
		{
			name:           "oracle-listener",
			sample:         "22-JUL-2019 15:16:13",
			expected:       time.Date(2019, time.July, 22, 15, 16, 13, 0, time.Local),
			gotimeLayout:   "02-Jan-2006 15:04:05",
			strptimeLayout: "%d-%b-%Y %H:%M:%S",
		},
		{
			name:           "k8s",
			sample:         "2019-03-08T18:41:12.152531115Z",
			expected:       time.Date(2019, time.March, 8, 18, 41, 12, 152531115, time.UTC),
			gotimeLayout:   "2006-01-02T15:04:05.999999999Z",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%sZ",
		},
		{
			name:           "k8s-override-location",
			sample:         "2019-03-08T18:41:12.152531115Z",
			expected:       time.Date(2019, time.March, 8, 18, 41, 12, 152531115, mst),
			gotimeLayout:   "2006-01-02T15:04:05.999999999Z",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%sZ",
			location:       mst.String(),
		},
		{
			name:           "jetty",
			sample:         "05/Aug/2019:20:38:46 +0000",
			expected:       time.Date(2019, time.August, 5, 20, 38, 46, 0, time.UTC),
			gotimeLayout:   "02/Jan/2006:15:04:05 -0700",
			strptimeLayout: "%d/%b/%Y:%H:%M:%S %z",
		},
		{
			name:           "puppet",
			sample:         "Aug  4 03:26:02",
			expected:       time.Date(timeutils.Now().Year(), time.August, 4, 3, 26, 02, 0, time.Local),
			gotimeLayout:   "Jan _2 15:04:05",
			strptimeLayout: "%b %e %H:%M:%S",
		},
		{
			name:           "esxi",
			sample:         "2020-12-16T21:43:28.391Z",
			expected:       time.Date(2020, 12, 16, 21, 43, 28, 391*1000*1000, time.UTC),
			gotimeLayout:   "2006-01-02T15:04:05.999Z",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%LZ",
		},
		{
			name:           "esxi-override-location",
			sample:         "2020-12-16T21:43:28.391Z",
			expected:       time.Date(2020, 12, 16, 21, 43, 28, 391*1000*1000, hst),
			gotimeLayout:   "2006-01-02T15:04:05.999Z",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%LZ",
			location:       hst.String(),
		},
		{
			name:           "1970",
			sample:         "1970-12-16T21:43:28.391Z",
			expected:       time.Date(1970, 12, 16, 21, 43, 28, 391*1000*1000, time.UTC),
			gotimeLayout:   "2006-01-02T15:04:05.999Z",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%LZ",
		},
	}

	rootField := entry.NewBodyField()
	someField := entry.NewBodyField("some_field")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotimeRootCfg := parseTimeTestConfig(GotimeKey, tc.gotimeLayout, tc.location, rootField)
			t.Run("gotime-root", runTimeParseTest(gotimeRootCfg, makeTestEntry(rootField, tc.sample), false, false, tc.expected))

			gotimeNonRootCfg := parseTimeTestConfig(GotimeKey, tc.gotimeLayout, tc.location, someField)
			t.Run("gotime-non-root", runTimeParseTest(gotimeNonRootCfg, makeTestEntry(someField, tc.sample), false, false, tc.expected))

			strptimeRootCfg := parseTimeTestConfig(StrptimeKey, tc.strptimeLayout, tc.location, rootField)
			t.Run("strptime-root", runTimeParseTest(strptimeRootCfg, makeTestEntry(rootField, tc.sample), false, false, tc.expected))

			strptimeNonRootCfg := parseTimeTestConfig(StrptimeKey, tc.strptimeLayout, tc.location, someField)
			t.Run("strptime-non-root", runTimeParseTest(strptimeNonRootCfg, makeTestEntry(someField, tc.sample), false, false, tc.expected))
		})
	}
}

func TestTimeEpochs(t *testing.T) {
	testCases := []struct {
		name     string
		sample   interface{}
		layout   string
		expected time.Time
		maxLoss  time.Duration
	}{
		{
			name:     "s-default-string",
			sample:   "1136214245",
			layout:   "s",
			expected: time.Unix(1136214245, 0),
		},
		{
			name:     "s-default-bytes",
			sample:   []byte("1136214245"),
			layout:   "s",
			expected: time.Unix(1136214245, 0),
		},
		{
			name:     "s-default-int",
			sample:   1136214245,
			layout:   "s",
			expected: time.Unix(1136214245, 0),
		},
		{
			name:     "s-default-float",
			sample:   1136214245.0,
			layout:   "s",
			expected: time.Unix(1136214245, 0),
		},
		{
			name:     "ms-default-string",
			sample:   "1136214245123",
			layout:   "ms",
			expected: time.Unix(1136214245, 123000000),
		},
		{
			name:     "ms-default-int",
			sample:   1136214245123,
			layout:   "ms",
			expected: time.Unix(1136214245, 123000000),
		},
		{
			name:     "ms-default-float",
			sample:   1136214245123.0,
			layout:   "ms",
			expected: time.Unix(1136214245, 123000000),
		},
		{
			name:     "us-default-string",
			sample:   "1136214245123456",
			layout:   "us",
			expected: time.Unix(1136214245, 123456000),
		},
		{
			name:     "us-default-int",
			sample:   1136214245123456,
			layout:   "us",
			expected: time.Unix(1136214245, 123456000),
		},
		{
			name:     "us-default-float",
			sample:   1136214245123456.0,
			layout:   "us",
			expected: time.Unix(1136214245, 123456000),
		},
		{
			name:     "ns-default-string",
			sample:   "1136214245123456789",
			layout:   "ns",
			expected: time.Unix(1136214245, 123456789),
		},
		{
			name:     "ns-default-int",
			sample:   1136214245123456789,
			layout:   "ns",
			expected: time.Unix(1136214245, 123456789),
		},
		{
			name:     "ns-default-float",
			sample:   1136214245123456789.0,
			layout:   "ns",
			expected: time.Unix(1136214245, 123456789),
			maxLoss:  time.Nanosecond * 100,
		},
		{
			name:     "s.ms-default-string",
			sample:   "1136214245.123",
			layout:   "s.ms",
			expected: time.Unix(1136214245, 123000000),
		},
		{
			name:     "s.ms-default-int",
			sample:   1136214245,
			layout:   "s.ms",
			expected: time.Unix(1136214245, 0), // drops subseconds
			maxLoss:  time.Nanosecond * 100,
		},
		{
			name:     "s.ms-default-float",
			sample:   1136214245.123,
			layout:   "s.ms",
			expected: time.Unix(1136214245, 123000000),
		},
		{
			name:     "s.us-default-string",
			sample:   "1136214245.123456",
			layout:   "s.us",
			expected: time.Unix(1136214245, 123456000),
		},
		{
			name:     "s.us-default-int",
			sample:   1136214245,
			layout:   "s.us",
			expected: time.Unix(1136214245, 0), // drops subseconds
			maxLoss:  time.Nanosecond * 100,
		},
		{
			name:     "s.us-default-float",
			sample:   1136214245.123456,
			layout:   "s.us",
			expected: time.Unix(1136214245, 123456000),
		},
		{
			name:     "s.ns-default-string",
			sample:   "1136214245.123456789",
			layout:   "s.ns",
			expected: time.Unix(1136214245, 123456789),
		},
		{
			name:     "s.ns-default-int",
			sample:   1136214245,
			layout:   "s.ns",
			expected: time.Unix(1136214245, 0), // drops subseconds
			maxLoss:  time.Nanosecond * 100,
		},
		{
			name:     "s.ns-default-float",
			sample:   1136214245.123456789,
			layout:   "s.ns",
			expected: time.Unix(1136214245, 123456789),
			maxLoss:  time.Nanosecond * 100,
		},
	}

	rootField := entry.NewBodyField()
	someField := entry.NewBodyField("some_field")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCfg := parseTimeTestConfig(EpochKey, tc.layout, "", rootField)
			t.Run("epoch-root", runLossyTimeParseTest(rootCfg, makeTestEntry(rootField, tc.sample), false, false, tc.expected, tc.maxLoss))

			nonRootCfg := parseTimeTestConfig(EpochKey, tc.layout, "", someField)
			t.Run("epoch-non-root", runLossyTimeParseTest(nonRootCfg, makeTestEntry(someField, tc.sample), false, false, tc.expected, tc.maxLoss))
		})
	}
}

func TestTimeErrors(t *testing.T) {
	testCases := []struct {
		name       string
		sample     interface{}
		layoutType string
		layout     string
		location   string
		buildErr   bool
		parseErr   bool
	}{
		{
			name:       "bad-layout-type",
			layoutType: "fake",
			buildErr:   true,
		},
		{
			name:       "bad-strptime-directive",
			layoutType: "strptime",
			layout:     "%1",
			buildErr:   true,
		},
		{
			name:       "bad-strptime-location",
			layoutType: "strptime",
			location:   "fake",
			buildErr:   true,
		},
		{
			name:       "bad-gotime-location",
			layoutType: "gotime",
			location:   "fake",
			buildErr:   true,
		},
		{
			name:       "bad-epoch-layout",
			layoutType: "epoch",
			layout:     "years",
			buildErr:   true,
		},
		{
			name:       "bad-native-value",
			layoutType: "native",
			sample:     1,
			parseErr:   true,
		},
		{
			name:       "bad-gotime-value",
			layoutType: "gotime",
			layout:     time.Kitchen,
			sample:     1,
			parseErr:   true,
		},
		{
			name:       "bad-epoch-value",
			layoutType: "epoch",
			layout:     "s",
			sample:     "not-a-number",
			parseErr:   true,
		},
	}

	rootField := entry.NewBodyField()
	someField := entry.NewBodyField("some_field")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCfg := parseTimeTestConfig(tc.layoutType, tc.layout, tc.location, rootField)
			t.Run("err-root", runTimeParseTest(rootCfg, makeTestEntry(rootField, tc.sample), tc.buildErr, tc.parseErr, time.Now()))

			nonRootCfg := parseTimeTestConfig(tc.layoutType, tc.layout, tc.location, someField)
			t.Run("err-non-root", runTimeParseTest(nonRootCfg, makeTestEntry(someField, tc.sample), tc.buildErr, tc.parseErr, time.Now()))
		})
	}
}

func runTimeParseTest(timeParser *TimeParser, ent *entry.Entry, buildErr bool, parseErr bool, expected time.Time) func(*testing.T) {
	return runLossyTimeParseTest(timeParser, ent, buildErr, parseErr, expected, time.Duration(0))
}

func runLossyTimeParseTest(timeParser *TimeParser, ent *entry.Entry, buildErr bool, parseErr bool, expected time.Time, maxLoss time.Duration) func(*testing.T) {
	return func(t *testing.T) {
		err := timeParser.Validate()
		if buildErr {
			require.Error(t, err, "expected error when validating config")
			return
		}
		require.NoError(t, err)

		err = timeParser.Parse(ent)
		if parseErr {
			require.Error(t, err, "expected error when configuring operator")
			return
		}
		require.NoError(t, err)

		if maxLoss == time.Duration(0) {
			require.True(t, expected.Equal(ent.Timestamp))
		} else {
			diff := time.Duration(math.Abs(float64(expected.Sub(ent.Timestamp))))
			require.True(t, diff <= maxLoss)
		}
	}
}

func parseTimeTestConfig(layoutType, layout, location string, parseFrom entry.Field) *TimeParser {
	return &TimeParser{
		LayoutType: layoutType,
		Layout:     layout,
		ParseFrom:  &parseFrom,
		Location:   location,
	}
}

func makeTestEntry(field entry.Field, value interface{}) *entry.Entry {
	e := entry.New()
	_ = e.Set(field, value)
	return e
}

func TestSetInvalidLocation(t *testing.T) {
	tp := NewTimeParser()
	tp.Location = "not_a_location"
	err := tp.setLocation()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load location "+"not_a_location")
}

func TestUnmarshalTimeConfig(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: newHelpersConfig(),
		TestsFile:     filepath.Join(".", "testdata", "timestamp.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "layout",
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Time = NewTimeParser()
					c.Time.Layout = "%Y-%m-%d"
					return c
				}(),
			},
			{
				Name: "layout_type",
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Time = NewTimeParser()
					c.Time.LayoutType = "epoch"
					return c
				}(),
			},
			{
				Name: "location",
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					c.Time = NewTimeParser()
					c.Time.Location = "America/Shiprock"
					return c
				}(),
			},
			{
				Name: "parse_from",
				Expect: func() *helpersConfig {
					c := newHelpersConfig()
					from := entry.NewBodyField("from")
					c.Time = NewTimeParser()
					c.Time.ParseFrom = &from
					return c
				}(),
			},
		},
	}.Run(t)
}
