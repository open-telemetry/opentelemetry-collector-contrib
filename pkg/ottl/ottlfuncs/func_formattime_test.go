// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_FormatTime(t *testing.T) {
	tests := []struct {
		name         string
		time         ottl.TimeGetter[any]
		format       string
		expected     string
		errorMsg     string
		funcErrorMsg string
	}{
		{
			name:     "empty format",
			time:     &ottl.StandardTimeGetter[any]{},
			format:   "",
			errorMsg: "format cannot be nil",
		},
		{
			name: "invalid time",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "something", nil
				},
			},
			format:       "%Y-%m-%d",
			funcErrorMsg: "expected time but got string",
		},
		{
			name: "simple short form",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2023, 4, 12, 0, 0, 0, 0, time.Local), nil
				},
			},
			format:   "%Y-%m-%d",
			expected: "2023-04-12",
		},
		{
			name: "simple short form with short year and slashes",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2011, 11, 11, 0, 0, 0, 0, time.Local), nil
				},
			},
			format:   "%d/%m/%y",
			expected: "11/11/11",
		},
		{
			name: "month day year",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2023, 2, 4, 0, 0, 0, 0, time.Local), nil
				},
			},
			format:   "%m/%d/%Y",
			expected: "02/04/2023",
		},
		{
			name: "simple long form",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(1993, 7, 31, 0, 0, 0, 0, time.Local), nil
				},
			},
			format:   "%B %d, %Y",
			expected: "July 31, 1993",
		},
		{
			name: "date with FormatTime",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2023, 3, 14, 17, 0o2, 59, 0, time.Local), nil
				},
			},
			format:   "%b %d %Y %H:%M:%S",
			expected: "Mar 14 2023 17:02:59",
		},
		{
			name: "day of the week long form",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2023, 5, 1, 0, 0, 0, 0, time.Local), nil
				},
			},
			format:   "%A, %B %d, %Y",
			expected: "Monday, May 01, 2023",
		},
		{
			name: "short weekday, short month, long format",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2023, 5, 20, 0, 0, 0, 0, time.Local), nil
				},
			},
			format:   "%a, %b %d, %Y",
			expected: "Sat, May 20, 2023",
		},
		{
			name: "short months",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2023, 2, 15, 0, 0, 0, 0, time.Local), nil
				},
			},
			format:   "%b %d, %Y",
			expected: "Feb 15, 2023",
		},
		{
			name: "simple short form with time",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2023, 5, 26, 12, 34, 56, 0, time.Local), nil
				},
			},
			format:   "%Y-%m-%d %H:%M:%S",
			expected: "2023-05-26 12:34:56",
		},
		{
			name: "RFC 3339 in custom format",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2012, 11, 0o1, 22, 8, 41, 0, time.Local), nil
				},
			},
			format:   "%Y-%m-%dT%H:%M:%S",
			expected: "2012-11-01T22:08:41",
		},
		{
			name: "RFC 3339 in custom format before 2000",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(1986, 10, 0o1, 0o0, 17, 33, 0o0, time.Local), nil
				},
			},
			format:   "%Y-%m-%dT%H:%M:%S",
			expected: "1986-10-01T00:17:33",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := FormatTime(tt.time, tt.format)
			if tt.errorMsg != "" {
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				result, err := exprFunc(nil, nil)
				if tt.funcErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.funcErrorMsg)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expected, result)
				}
			}
		})
	}
}
