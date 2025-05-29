// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Time(t *testing.T) {
	locationAmericaNewYork, _ := time.LoadLocation("America/New_York")
	locationAsiaShanghai, _ := time.LoadLocation("Asia/Shanghai")

	tests := []struct {
		name     string
		time     ottl.StringGetter[any]
		format   string
		expected time.Time
		location string
		locale   string
	}{
		{
			name: "simple short form",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-04-12", nil
				},
			},
			format:   "%Y-%m-%d",
			expected: time.Date(2023, 4, 12, 0, 0, 0, 0, time.Local),
		},
		{
			name: "simple short form with short year and slashes",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "11/11/11", nil
				},
			},
			format:   "%d/%m/%y",
			expected: time.Date(2011, 11, 11, 0, 0, 0, 0, time.Local),
		},
		{
			name: "month day year",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "02/04/2023", nil
				},
			},
			format:   "%m/%d/%Y",
			expected: time.Date(2023, 2, 4, 0, 0, 0, 0, time.Local),
		},
		{
			name: "simple long form",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "July 31, 1993", nil
				},
			},
			format:   "%B %d, %Y",
			expected: time.Date(1993, 7, 31, 0, 0, 0, 0, time.Local),
		},
		{
			name: "date with timestamp",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Mar 14 2023 17:02:59", nil
				},
			},
			format:   "%b %d %Y %H:%M:%S",
			expected: time.Date(2023, 3, 14, 17, 0o2, 59, 0, time.Local),
		},
		{
			name: "day of the week long form",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Monday, May 01, 2023", nil
				},
			},
			format:   "%A, %B %d, %Y",
			expected: time.Date(2023, 5, 1, 0, 0, 0, 0, time.Local),
		},
		{
			name: "short weekday, short month, long format",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Sat, May 20, 2023", nil
				},
			},
			format:   "%a, %b %d, %Y",
			expected: time.Date(2023, 5, 20, 0, 0, 0, 0, time.Local),
		},
		{
			name: "short months",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Feb 15, 2023", nil
				},
			},
			format:   "%b %d, %Y",
			expected: time.Date(2023, 2, 15, 0, 0, 0, 0, time.Local),
		},
		{
			name: "timestamp with time zone offset",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-05-26 12:34:56 HST", nil
				},
			},
			format:   "%Y-%m-%d %H:%M:%S %Z",
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, time.FixedZone("HST", -10*60*60)),
		},
		{
			name: "short date with timestamp without time zone offset",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-05-26T12:34:56 GMT", nil
				},
			},
			format:   "%Y-%m-%dT%H:%M:%S %Z",
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, time.FixedZone("GMT", 0)),
		},
		{
			name: "RFC 3339 in custom format",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2012-11-01T22:08:41+0000 EST", nil
				},
			},
			format:   "%Y-%m-%dT%H:%M:%S%z %Z",
			expected: time.Date(2012, 11, 0o1, 22, 8, 41, 0, time.FixedZone("EST", 0)),
		},
		{
			name: "RFC 3339 in custom format before 2000",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "1986-10-01T00:17:33 MST", nil
				},
			},
			format:   "%Y-%m-%dT%H:%M:%S %Z",
			expected: time.Date(1986, 10, 0o1, 0o0, 17, 33, 0o0, time.FixedZone("MST", -7*60*60)),
		},
		{
			name: "no location",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2022/01/01", nil
				},
			},
			format:   "%Y/%m/%d",
			expected: time.Date(2022, 0o1, 0o1, 0, 0, 0, 0, time.Local),
		},
		{
			name: "with location - America",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-05-26 12:34:56", nil
				},
			},
			format:   "%Y-%m-%d %H:%M:%S",
			location: "America/New_York",
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, locationAmericaNewYork),
		},
		{
			name: "with location - Asia",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-05-26 12:34:56", nil
				},
			},
			format:   "%Y-%m-%d %H:%M:%S",
			location: "Asia/Shanghai",
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, locationAsiaShanghai),
		},
		{
			name: "RFC 3339 in custom format before 2000, ignore default location",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "1986-10-01T00:17:33 MST", nil
				},
			},
			location: "Asia/Shanghai",
			format:   "%Y-%m-%dT%H:%M:%S %Z",
			expected: time.Date(1986, 10, 0o1, 0o0, 17, 33, 0o0, time.FixedZone("MST", -7*60*60)),
		},
		{
			name: "with locale",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Febrero 25 lunes, 2002, 02:03:04 p.m.", nil
				},
			},
			format:   "%B %d %A, %Y, %r",
			locale:   "es-ES",
			expected: time.Date(2002, 2, 25, 14, 0o3, 0o4, 0, time.Local),
		},
		{
			name: "with locale - date only",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "mercoledì set 4 2024", nil
				},
			},
			format:   "%A %h %e %Y",
			locale:   "it",
			expected: time.Date(2024, 9, 4, 0, 0, 0, 0, time.Local),
		},
		{
			name: "with locale and location",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Febrero 25 lunes, 2002, 02:03:04 p.m.", nil
				},
			},
			format:   "%B %d %A, %Y, %r",
			location: "America/New_York",
			locale:   "es-ES",
			expected: time.Date(2002, 2, 25, 14, 0o3, 0o4, 0, locationAmericaNewYork),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var locationOptional ottl.Optional[string]
			if tt.location != "" {
				locationOptional = ottl.NewTestingOptional(tt.location)
			}
			var localeOptional ottl.Optional[string]
			if tt.locale != "" {
				localeOptional = ottl.NewTestingOptional(tt.locale)
			}
			exprFunc, err := Time(tt.time, tt.format, locationOptional, localeOptional)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.UnixNano(), result.(time.Time).UnixNano())
		})
	}
}

func Test_TimeError(t *testing.T) {
	tests := []struct {
		name          string
		time          ottl.StringGetter[any]
		format        string
		expectedError string
	}{
		{
			name: "invalid short format",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "11/11/11", nil
				},
			},
			format:        "%Y/%m/%d",
			expectedError: `parsing time "11/11/11" as "%Y/%m/%d": cannot parse "11/11/11" as "%Y"`,
		},
		{
			name: "invalid RFC3339 with no time",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "", nil
				},
			},
			format:        "%Y-%m-%dT%H:%M:%S",
			expectedError: "time cannot be nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var locationOptional ottl.Optional[string]
			var localeOptional ottl.Optional[string]
			exprFunc, err := Time[any](tt.time, tt.format, locationOptional, localeOptional)
			require.NoError(t, err)
			_, err = exprFunc(context.Background(), nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func Test_TimeFormatError(t *testing.T) {
	tests := []struct {
		name          string
		time          ottl.StringGetter[any]
		format        string
		expectedError string
		location      string
		locale        string
	}{
		{
			name: "invalid short with no format",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "11/11/11", nil
				},
			},
			format:        "",
			expectedError: "format cannot be nil",
		},
		{
			name: "with unknown location",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-05-26 12:34:56", nil
				},
			},
			format:        "%Y-%m-%d %H:%M:%S",
			location:      "Jupiter/Ganymede",
			expectedError: "unknown time zone Jupiter/Ganymede",
		},
		{
			name: "with unsupported locale",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-05-26 12:34:56", nil
				},
			},
			format:        "%Y-%m-%d %H:%M:%S",
			locale:        "foo-bar",
			expectedError: "unsupported locale 'foo-bar'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var locationOptional ottl.Optional[string]
			if tt.location != "" {
				locationOptional = ottl.NewTestingOptional(tt.location)
			}
			var localeOptional ottl.Optional[string]
			if tt.locale != "" {
				localeOptional = ottl.NewTestingOptional(tt.locale)
			}
			_, err := Time[any](tt.time, tt.format, locationOptional, localeOptional)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func Benchmark_Time(t *testing.B) {
	locationAmericaNewYork, _ := time.LoadLocation("America/New_York")
	locationAsiaShanghai, _ := time.LoadLocation("Asia/Shanghai")

	tests := []struct {
		name     string
		time     ottl.StringGetter[any]
		format   string
		expected time.Time
		location string
	}{
		{
			name: "simple short form",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-04-12", nil
				},
			},
			format:   "%Y-%m-%d",
			expected: time.Date(2023, 4, 12, 0, 0, 0, 0, time.Local),
		},
		{
			name: "simple short form with short year and slashes",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "11/11/11", nil
				},
			},
			format:   "%d/%m/%y",
			expected: time.Date(2011, 11, 11, 0, 0, 0, 0, time.Local),
		},
		{
			name: "month day year",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "02/04/2023", nil
				},
			},
			format:   "%m/%d/%Y",
			expected: time.Date(2023, 2, 4, 0, 0, 0, 0, time.Local),
		},
		{
			name: "simple long form",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "July 31, 1993", nil
				},
			},
			format:   "%B %d, %Y",
			expected: time.Date(1993, 7, 31, 0, 0, 0, 0, time.Local),
		},
		{
			name: "date with timestamp",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Mar 14 2023 17:02:59", nil
				},
			},
			format:   "%b %d %Y %H:%M:%S",
			expected: time.Date(2023, 3, 14, 17, 0o2, 59, 0, time.Local),
		},
		{
			name: "day of the week long form",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Monday, May 01, 2023", nil
				},
			},
			format:   "%A, %B %d, %Y",
			expected: time.Date(2023, 5, 1, 0, 0, 0, 0, time.Local),
		},
		{
			name: "short weekday, short month, long format",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Sat, May 20, 2023", nil
				},
			},
			format:   "%a, %b %d, %Y",
			expected: time.Date(2023, 5, 20, 0, 0, 0, 0, time.Local),
		},
		{
			name: "short months",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "Feb 15, 2023", nil
				},
			},
			format:   "%b %d, %Y",
			expected: time.Date(2023, 2, 15, 0, 0, 0, 0, time.Local),
		},
		{
			name: "timestamp with time zone offset",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-05-26 12:34:56 HST", nil
				},
			},
			format:   "%Y-%m-%d %H:%M:%S %Z",
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, time.FixedZone("HST", -10*60*60)),
		},
		{
			name: "short date with timestamp without time zone offset",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-05-26T12:34:56 GMT", nil
				},
			},
			format:   "%Y-%m-%dT%H:%M:%S %Z",
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, time.FixedZone("GMT", 0)),
		},
		{
			name: "RFC 3339 in custom format",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2012-11-01T22:08:41+0000 EST", nil
				},
			},
			format:   "%Y-%m-%dT%H:%M:%S%z %Z",
			expected: time.Date(2012, 11, 0o1, 22, 8, 41, 0, time.FixedZone("EST", 0)),
		},
		{
			name: "RFC 3339 in custom format before 2000",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "1986-10-01T00:17:33 MST", nil
				},
			},
			format:   "%Y-%m-%dT%H:%M:%S %Z",
			expected: time.Date(1986, 10, 0o1, 0o0, 17, 33, 0o0, time.FixedZone("MST", -7*60*60)),
		},
		{
			name: "no location",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2022/01/01", nil
				},
			},
			format:   "%Y/%m/%d",
			expected: time.Date(2022, 0o1, 0o1, 0, 0, 0, 0, time.Local),
		},
		{
			name: "with location - America",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-05-26 12:34:56", nil
				},
			},
			format:   "%Y-%m-%d %H:%M:%S",
			location: "America/New_York",
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, locationAmericaNewYork),
		},
		{
			name: "with location - Asia",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "2023-05-26 12:34:56", nil
				},
			},
			format:   "%Y-%m-%d %H:%M:%S",
			location: "Asia/Shanghai",
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, locationAsiaShanghai),
		},
		{
			name: "RFC 3339 in custom format before 2000, ignore default location",
			time: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "1986-10-01T00:17:33 MST", nil
				},
			},
			location: "Asia/Shanghai",
			format:   "%Y-%m-%dT%H:%M:%S %Z",
			expected: time.Date(1986, 10, 0o1, 0o0, 17, 33, 0o0, time.FixedZone("MST", -7*60*60)),
		},
	}
	for _, tt := range tests {
		var locOptional ottl.Optional[string]
		if tt.location != "" {
			locOptional = ottl.NewTestingOptional(tt.location)
		}
		exprFunc, err := Time(tt.time, tt.format, locOptional, ottl.Optional[string]{})
		assert.NoError(t, err)

		t.Run(tt.name, func(t *testing.B) {
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.UnixNano(), result.(time.Time).UnixNano())
		})
	}
}
