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

func Test_time(t *testing.T) {
	tests := []struct {
		name     string
		time     ottl.StringGetter[interface{}]
		format   ottl.StringGetter[interface{}]
		expected interface{}
	}{
		{
			name: "simple short form",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2023-04-12", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2006-01-02", nil
				},
			},
			expected: time.Date(2023, 4, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "simple short form with short year and slashes",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "11/11/11", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "01/02/06", nil
				},
			},
			expected: time.Date(2011, 11, 11, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "single-digit month and day",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2/4/2023", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1/2/2006", nil
				},
			},
			expected: time.Date(2023, 2, 4, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "simple short form with long year and slashes",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "02/12/2022", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "01/02/2006", nil
				},
			},
			expected: time.Date(2022, 2, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "simple long form",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "July 31, 1993", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "January 2, 2006", nil
				},
			},
			expected: time.Date(1993, 7, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "date with timestamp",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Mar 14 2023 17:02:59", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Jan _2 2006 15:04:05", nil
				},
			},
			expected: time.Date(2023, 3, 14, 17, 02, 59, 0, time.UTC),
		},
		{
			name: "day of the week long form",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Monday, May 1, 2023", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Monday, January 2, 2006", nil
				},
			},
			expected: time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "short weekday, short month, long format",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Sat, May 20, 2023", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Mon, Jan 2, 2006", nil
				},
			},
			expected: time.Date(2023, 5, 20, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "short months",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Feb 15, 2023", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "Jan 2, 2006", nil
				},
			},
			expected: time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "timestamp with time zone offset",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2023-05-26T12:34:56+02:00", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2006-01-02T15:04:05-07:00", nil
				},
			},
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, time.FixedZone("", 2*60*60)),
		},
		{
			name: "short date with timestamp without time zone offset",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2023-05-26T12:34:56", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2006-01-02T15:04:05", nil
				},
			},
			expected: time.Date(2023, 5, 26, 12, 34, 56, 0, time.UTC),
		},
		{
			name: "RFC 3339 in custom format",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2012-11-01T22:08:41+00:00", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2006-01-02T15:04:05+00:00", nil
				},
			},
			expected: time.Date(2012, 11, 01, 22, 8, 41, 0, time.UTC),
		},
		{
			name: "RFC 3339 in custom format",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1986-10-01T00:17:33Z", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "RFC3339", nil
				},
			},
			expected: time.Date(1986, 10, 01, 00, 17, 33, 00, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := toTime(tt.time, tt.format)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_timeError(t *testing.T) {
	tests := []struct {
		name          string
		time          ottl.StringGetter[interface{}]
		format        ottl.StringGetter[interface{}]
		expectedError string
	}{
		{
			name: "invalid short format",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "11/11/11", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "01-02/06", nil
				},
			},
			expectedError: "cannot parse \"/11/11\" as \"-\"",
		},
		{
			name: "invalid short with no format",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "11/11/11", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			expectedError: "extra text: \"11/11/11\"",
		},
		{
			name: "invalid RFC3339 with no time",
			time: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			format: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "RFC3339", nil
				},
			},
			expectedError: "time cannot be nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := toTime[any](tt.time, tt.format)
			require.NoError(t, err)
			_, err = exprFunc(context.Background(), nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}
