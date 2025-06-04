// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveIfFloat64(t *testing.T) {
	tests := []struct {
		name      string
		input     []float64
		predicate func(float64, int) bool
		want      []float64
	}{
		{
			name:  "remove even numbers",
			input: []float64{1, 2, 3, 4, 5},
			predicate: func(v float64, _ int) bool {
				return int(v)%2 == 0
			},
			want: []float64{1, 3, 5},
		},
		{
			name:  "remove nothing",
			input: []float64{1, 2, 3},
			predicate: func(_ float64, _ int) bool {
				return false
			},
			want: []float64{1, 2, 3},
		},
		{
			name:  "remove all",
			input: []float64{1, 2, 3},
			predicate: func(_ float64, _ int) bool {
				return true
			},
			want: []float64{},
		},
		{
			name:  "empty slice",
			input: []float64{},
			predicate: func(_ float64, _ int) bool {
				return true
			},
			want: []float64{},
		},
		{
			name:  "remove by index",
			input: []float64{1, 2, 3, 4, 5},
			predicate: func(_ float64, idx int) bool {
				return idx == 1 || idx == 3
			},
			want: []float64{1, 3, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]float64, len(tt.input))
			copy(input, tt.input)
			got := RemoveIfFloat64(input, tt.predicate)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRemoveIfUint64(t *testing.T) {
	tests := []struct {
		name      string
		input     []uint64
		predicate func(uint64, int) bool
		want      []uint64
	}{
		{
			name:  "remove even numbers",
			input: []uint64{1, 2, 3, 4, 5},
			predicate: func(v uint64, _ int) bool {
				return v%2 == 0
			},
			want: []uint64{1, 3, 5},
		},
		{
			name:  "remove nothing",
			input: []uint64{1, 2, 3},
			predicate: func(_ uint64, _ int) bool {
				return false
			},
			want: []uint64{1, 2, 3},
		},
		{
			name:  "remove all",
			input: []uint64{1, 2, 3},
			predicate: func(_ uint64, _ int) bool {
				return true
			},
			want: []uint64{},
		},
		{
			name:  "empty slice",
			input: []uint64{},
			predicate: func(_ uint64, _ int) bool {
				return true
			},
			want: []uint64{},
		},
		{
			name:  "remove by index",
			input: []uint64{1, 2, 3, 4, 5},
			predicate: func(_ uint64, idx int) bool {
				return idx == 1 || idx == 3
			},
			want: []uint64{1, 3, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]uint64, len(tt.input))
			copy(input, tt.input)
			got := RemoveIfUint64(input, tt.predicate)
			assert.Equal(t, tt.want, got)
		})
	}
}
