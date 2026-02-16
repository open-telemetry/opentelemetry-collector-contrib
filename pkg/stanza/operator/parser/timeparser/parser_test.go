// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package timeparser

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestIsZero(t *testing.T) {
	require.True(t, (&helper.TimeParser{}).IsZero())
	require.False(t, (&helper.TimeParser{Layout: "strptime"}).IsZero())
}

func TestInit(t *testing.T) {
	builder, ok := operator.DefaultRegistry.Lookup("time_parser")
	require.True(t, ok, "expected time_parser to be registered")
	require.Equal(t, "time_parser", builder().Type())
}

func TestBuild(t *testing.T) {
	testCases := []struct {
		name      string
		input     func() (*Config, error)
		expectErr bool
	}{
		{
			"empty",
			func() (*Config, error) {
				return &Config{}, nil
			},
			true,
		},
		{
			"basic",
			func() (*Config, error) {
				cfg := NewConfigWithID("test_id")
				parseFrom, err := entry.NewField("body.app_time")
				if err != nil {
					return cfg, err
				}
				cfg.ParseFrom = &parseFrom
				cfg.LayoutType = "gotime"
				cfg.Layout = "Mon Jan 2 15:04:05 MST 2006"
				return cfg, nil
			},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := tc.input()
			require.NoError(t, err, "expected nil error when running test cases input func")
			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			if tc.expectErr {
				require.Error(t, err, "expected error while building time_parser operator")
				return
			}
			require.NoError(t, err, "did not expect error while building time_parser operator")
			require.NotNil(t, op, "expected Build to return an operator")
		})
	}
}

func TestProcess(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name   string
		config func() (*Config, error)
		input  *entry.Entry
		expect *entry.Entry
	}{
		{
			name: "promote",
			config: func() (*Config, error) {
				cfg := NewConfigWithID("test_id")
				parseFrom, err := entry.NewField("body.app_time")
				if err != nil {
					return nil, err
				}
				cfg.ParseFrom = &parseFrom
				cfg.LayoutType = "gotime"
				cfg.Layout = "Mon Jan 2 15:04:05 MST 2006"
				return cfg, nil
			},
			input: func() *entry.Entry {
				e := entry.New()
				e.ObservedTimestamp = now
				e.Body = map[string]any{
					"app_time": "Mon Jan 2 15:04:05 UTC 2006",
				}
				return e
			}(),
			expect: &entry.Entry{
				ObservedTimestamp: now,
				Timestamp:         time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC),
				Body: map[string]any{
					"app_time": "Mon Jan 2 15:04:05 UTC 2006",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := tc.config()
			if err != nil {
				require.NoError(t, err)
				return
			}
			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			if err != nil {
				require.NoError(t, err)
				return
			}

			require.True(t, op.CanOutput(), "expected test operator CanOutput to return true")

			err = op.Process(t.Context(), tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expect, tc.input)
		})
	}
}

func TestTimeParser(t *testing.T) {
	// Mountain Standard Time
	mst, err := time.LoadLocation("MST")
	require.NoError(t, err)

	// Hawaiian Standard Time
	hst, err := time.LoadLocation("HST")
	require.NoError(t, err)

	testCases := []struct {
		name           string
		sample         any
		expected       time.Time
		gotimeLayout   string
		strptimeLayout string
	}{
		{
			name:           "unix-utc",
			sample:         "Mon Jan 2 15:04:05 UTC 2006",
			expected:       time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC),
			gotimeLayout:   "Mon Jan 2 15:04:05 MST 2006",
			strptimeLayout: "%a %b %e %H:%M:%S %Z %Y",
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
			name:           "opendistro",
			sample:         "2020-06-09T15:39:58",
			expected:       time.Date(2020, time.June, 9, 15, 39, 58, 0, time.Local),
			gotimeLayout:   "2006-01-02T15:04:05",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S",
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
			name:           "cassandra",
			sample:         "2019-11-27T09:34:32.901-1000",
			expected:       time.Date(2019, time.November, 27, 9, 34, 32, 901*1000*1000, hst),
			gotimeLayout:   "2006-01-02T15:04:05.999-0700",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%L%z",
		},
		{
			name:           "oracle",
			sample:         "2019-10-15T10:42:01.900436-10:00",
			expected:       time.Date(2019, time.October, 15, 10, 42, 0o1, 900436*1000, hst),
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
			name:           "jetty",
			sample:         "05/Aug/2019:20:38:46 +0000",
			expected:       time.Date(2019, time.August, 5, 20, 38, 46, 0, time.UTC),
			gotimeLayout:   "02/Jan/2006:15:04:05 -0700",
			strptimeLayout: "%d/%b/%Y:%H:%M:%S %z",
		},
		{
			name:           "esxi",
			sample:         "2020-12-16T21:43:28.391Z",
			expected:       time.Date(2020, 12, 16, 21, 43, 28, 391*1000*1000, time.UTC),
			gotimeLayout:   "2006-01-02T15:04:05.999Z",
			strptimeLayout: "%Y-%m-%dT%H:%M:%S.%LZ",
		},
	}

	rootField := entry.NewBodyField()
	someField := entry.NewBodyField("some_field")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotimeRootCfg := parseTimeTestConfig(helper.GotimeKey, tc.gotimeLayout, rootField)
			t.Run("gotime-root", runTimeParseTest(t, gotimeRootCfg, makeTestEntry(t, rootField, tc.sample), false, false, tc.expected))

			gotimeNonRootCfg := parseTimeTestConfig(helper.GotimeKey, tc.gotimeLayout, someField)
			t.Run("gotime-non-root", runTimeParseTest(t, gotimeNonRootCfg, makeTestEntry(t, someField, tc.sample), false, false, tc.expected))

			strptimeRootCfg := parseTimeTestConfig(helper.StrptimeKey, tc.strptimeLayout, rootField)
			t.Run("strptime-root", runTimeParseTest(t, strptimeRootCfg, makeTestEntry(t, rootField, tc.sample), false, false, tc.expected))

			strptimeNonRootCfg := parseTimeTestConfig(helper.StrptimeKey, tc.strptimeLayout, someField)
			t.Run("strptime-non-root", runTimeParseTest(t, strptimeNonRootCfg, makeTestEntry(t, someField, tc.sample), false, false, tc.expected))
		})
	}
}

func TestTimeEpochs(t *testing.T) {
	testCases := []struct {
		name     string
		sample   any
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
			rootCfg := parseTimeTestConfig(helper.EpochKey, tc.layout, rootField)
			t.Run("epoch-root", runLossyTimeParseTest(t, rootCfg, makeTestEntry(t, rootField, tc.sample), false, false, tc.expected, tc.maxLoss))

			nonRootCfg := parseTimeTestConfig(helper.EpochKey, tc.layout, someField)
			t.Run("epoch-non-root", runLossyTimeParseTest(t, nonRootCfg, makeTestEntry(t, someField, tc.sample), false, false, tc.expected, tc.maxLoss))
		})
	}
}

func TestTimeErrors(t *testing.T) {
	testCases := []struct {
		name       string
		sample     any
		layoutType string
		layout     string
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
			rootCfg := parseTimeTestConfig(tc.layoutType, tc.layout, rootField)
			t.Run("err-root", runTimeParseTest(t, rootCfg, makeTestEntry(t, rootField, tc.sample), tc.buildErr, tc.parseErr, time.Now()))

			nonRootCfg := parseTimeTestConfig(tc.layoutType, tc.layout, someField)
			t.Run("err-non-root", runTimeParseTest(t, nonRootCfg, makeTestEntry(t, someField, tc.sample), tc.buildErr, tc.parseErr, time.Now()))
		})
	}
}

func makeTestEntry(t *testing.T, field entry.Field, value any) *entry.Entry {
	e := entry.New()
	require.NoError(t, e.Set(field, value))
	return e
}

func runTimeParseTest(t *testing.T, cfg *Config, ent *entry.Entry, buildErr, parseErr bool, expected time.Time) func(*testing.T) {
	return runLossyTimeParseTest(t, cfg, ent, buildErr, parseErr, expected, time.Duration(0))
}

func runLossyTimeParseTest(_ *testing.T, cfg *Config, ent *entry.Entry, buildErr, parseErr bool, expected time.Time, maxLoss time.Duration) func(*testing.T) {
	return func(t *testing.T) {
		set := componenttest.NewNopTelemetrySettings()
		op, err := cfg.Build(set)
		if buildErr {
			require.Error(t, err, "expected error when configuring operator")
			return
		}
		require.NoError(t, err)

		mockOutput := &testutil.Operator{}
		resultChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			resultChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		timeParser := op.(*Parser)
		timeParser.OutputOperators = []operator.Operator{mockOutput}

		ots := ent.ObservedTimestamp
		err = timeParser.Parse(ent)
		if parseErr {
			require.Error(t, err, "expected error when configuring operator")
			return
		}
		require.NoError(t, err)
		require.Equal(t, ots, ent.ObservedTimestamp, "time parsing should not change observed timestamp")

		diff := time.Duration(math.Abs(float64(expected.Sub(ent.Timestamp))))
		require.LessOrEqual(t, diff, maxLoss)
	}
}

func parseTimeTestConfig(layoutType, layout string, parseFrom entry.Field) *Config {
	cfg := NewConfigWithID("test_operator_id")
	cfg.OutputIDs = []string{"output1"}
	cfg.TimeParser = helper.TimeParser{
		LayoutType: layoutType,
		Layout:     layout,
		ParseFrom:  &parseFrom,
	}
	return cfg
}
