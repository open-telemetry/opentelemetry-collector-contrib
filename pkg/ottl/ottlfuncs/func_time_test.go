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
	tests := []struct {
		name     string
		time     ottl.StringGetter[interface{}]
		format   ottl.StringGetter[interface{}]
		location ottl.StringGetter[interface{}]
		expected time.Time
	}{
		{
			name: "simple short form",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2023-04-12", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%Y-%m-%d", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "UTC", nil
				},
			},
			expected: time.Date(2023, 4, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "simple short form with short year and slashes",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "11/11/11", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%d/%m/%y", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "UTC", nil
				},
			},
			expected: time.Date(2011, 11, 11, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "month day year",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "02/04/2023", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%m/%d/%Y", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "UTC", nil
				},
			},
			expected: time.Date(2023, 2, 4, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "simple short form with long year and slashes",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "02/12/2022", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%m/%d/%Y", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "UTC", nil
				},
			},
			expected: time.Date(2022, 2, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "simple long form",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "July 31, 1993", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%B %d, %Y", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "UTC", nil
				},
			},
			expected: time.Date(1993, 7, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "date with timestamp",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Mar 14 2023 17:02:59", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%b %d %Y %H:%M:%S", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "UTC", nil
				},
			},
			expected: time.Date(2023, 3, 14, 17, 02, 59, 0, time.UTC),
		},
		{
			name: "day of the week long form",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Monday, May 01, 2023", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%A, %B %d, %Y", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "UTC", nil
				},
			},
			expected: time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "short weekday, short month, long format",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Sat, May 20, 2023", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%a, %b %d, %Y", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "UTC", nil
				},
			},
			expected: time.Date(2023, 5, 20, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "short months",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Feb 15, 2023", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%b %d, %Y", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "UTC", nil
				},
			},
			expected: time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "timestamp with time zone offset",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2023-05-26 12:34:56 HST", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%Y-%m-%d %H:%M:%S %Z", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "HST", nil
				},
			},
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, time.FixedZone("HST", -10*60*60)),
		},
		{
			name: "short date with timestamp without time zone offset",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2023-05-26T12:34:56", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%Y-%m-%dT%H:%M:%S", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "GMT", nil
				},
			},
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, time.FixedZone("GMT", 0)),
		},
		{
			name: "RFC 3339 in custom format",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2012-11-01T22:08:41+0000", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%Y-%m-%dT%H:%M:%S%z", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "EST", nil
				},
			},
			expected: time.Date(2012, 11, 01, 22, 8, 41, 0, time.FixedZone("EST", 0)),
		},
		{
			name: "RFC 3339 in custom format before 2000",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1986-10-01T00:17:33", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%Y-%m-%dT%H:%M:%S", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "MST", nil
				},
			},
			expected: time.Date(1986, 10, 01, 00, 17, 33, 00, time.FixedZone("MST", -7*60*60)),
		},
		{
			name: "empty location",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2022/01/01", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%Y/%m/%d", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			expected: time.Date(2022, 01, 01, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Time(tt.time, tt.format, tt.location)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.True(t, tt.expected.Equal(result.(time.Time)))
		})
	}
}

func Test_TimeError(t *testing.T) {
	tests := []struct {
		name          string
		time          ottl.StringGetter[interface{}]
		format        ottl.StringGetter[interface{}]
		location      ottl.StringGetter[interface{}]
		expectedError string
	}{
		{
			name: "invalid short format",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "11/11/11", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%Y/%m/%d", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "UTC", nil
				},
			},
			expectedError: "cannot parse \"1/11\"",
		},
		{
			name: "invalid short with no format",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "11/11/11", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "CST", nil
				},
			},
			expectedError: "format cannot be nil",
		},
		{
			name: "invalid RFC3339 with no time",
			time: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			format: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "%Y-%m-%dT%H:%M:%S", nil
				},
			},
			location: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "GMT", nil
				},
			},
			expectedError: "time cannot be nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Time[any](tt.time, tt.format, tt.location)
			require.NoError(t, err)
			_, err = exprFunc(context.Background(), nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}
