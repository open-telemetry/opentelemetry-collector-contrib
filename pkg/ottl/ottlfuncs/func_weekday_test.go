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

func Test_Weekday(t *testing.T) {
	tests := []struct {
		name     string
		time     ottl.TimeGetter[any]
		expected int64
	}{
		{
			name: "Mon",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2025, time.February, 24, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 1,
		},
		{
			name: "Tue",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2025, time.February, 25, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 2,
		},
		{
			name: "Wed",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2025, time.February, 26, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 3,
		},
		{
			name: "Thu",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2025, time.February, 27, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 4,
		},
		{
			name: "Fri",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2025, time.February, 28, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 5,
		},
		{
			name: "Sat",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2025, time.February, 22, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 6,
		},
		{
			name: "Sun",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2025, time.February, 23, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Weekday(tt.time)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Weekday_Error(t *testing.T) {
	var getter ottl.TimeGetter[any] = &ottl.StandardTimeGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "not a time", nil
		},
	}
	exprFunc, err := Weekday(getter)
	assert.NoError(t, err)
	result, err := exprFunc(context.Background(), nil)
	assert.Nil(t, result)
	assert.Error(t, err)
}
