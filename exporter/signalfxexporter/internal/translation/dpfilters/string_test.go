// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dpfilters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringFilter(t *testing.T) {
	tests := []struct {
		name        string
		filter      []string
		inputs      []string
		shouldMatch []bool
		shouldError bool
	}{
		{
			filter:      []string{},
			inputs:      []string{"process_", "", "asdf"},
			shouldMatch: []bool{false, false, false},
		},
		{
			filter: []string{
				"*",
			},
			inputs:      []string{"app", "asdf", "", "*"},
			shouldMatch: []bool{true, true, true, true},
		},
		{
			filter: []string{
				"!app",
			},
			inputs:      []string{"app", "other"},
			shouldMatch: []bool{false, false},
		},
		{
			filter: []string{
				"app",
				"!app",
			},
			inputs:      []string{"app", "other"},
			shouldMatch: []bool{false, false},
		},
		{
			filter: []string{
				"other",
				"!app",
			},
			inputs:      []string{"other", "something", "app"},
			shouldMatch: []bool{true, false, false},
		},
		{
			filter: []string{
				"/^process_/",
				"/^node_/",
			},
			inputs:      []string{"process_", "node_", "process_asdf", "other"},
			shouldMatch: []bool{true, true, true, false},
		},
		{
			filter: []string{
				"!/^process_/",
			},
			inputs:      []string{"process_", "other"},
			shouldMatch: []bool{false, false},
		},
		{
			filter: []string{
				"app",
				"!/^process_/",
				"process_",
			},
			inputs:      []string{"other", "app", "process_cpu", "process_"},
			shouldMatch: []bool{false, true, false, false},
		},
		{
			filter: []string{
				"asdfdfasdf",
				"/^node_/",
			},
			inputs:      []string{"node_test"},
			shouldMatch: []bool{true},
		},
		{
			filter: []string{
				"process_*",
				"!process_cpu",
			},
			inputs:      []string{"process_mem", "process_cpu", "asdf"},
			shouldMatch: []bool{true, false, false},
		},
		{
			filter: []string{
				"*",
				"!process_cpu",
			},
			inputs:      []string{"process_mem", "process_cpu", "asdf"},
			shouldMatch: []bool{true, false, true},
		},
		{
			filter: []string{
				"metric_?",
				"!metric_a",
				"!metric_b",
				"random",
			},
			inputs:      []string{"metric_a", "metric_b", "metric_c", "asdf", "random"},
			shouldMatch: []bool{false, false, true, false, true},
		},
		{
			filter: []string{
				"!process_cpu",
				// Order doesn't matter
				"*",
			},
			inputs:      []string{"process_mem", "process_cpu", "asdf"},
			shouldMatch: []bool{true, false, true},
		},
		{
			filter: []string{
				"/a.*/",
				"!/.*z/",
				"b",
				// Static match should not override the negated regex above
				"alz",
			},
			inputs:      []string{"", "asdf", "asdz", "b", "wrong", "alz"},
			shouldMatch: []bool{false, true, false, true, false, false},
		},
		{
			filter:      []string{"!memory*"},
			inputs:      []string{"cpu.utilization", "memory.utilization"},
			shouldMatch: []bool{false, false},
		},
		{
			filter:      []string{"/!memor*(/"},
			shouldError: true,
		},
		{
			filter:      nil,
			inputs:      []string{"cpu.utilization", "memory.utilization"},
			shouldMatch: []bool{false, false},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, err := NewStringFilter(test.filter)
			if test.shouldError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			for i := range test.inputs {
				assert.Equal(t, test.shouldMatch[i], f.Matches(test.inputs[i]))
			}
		})
	}
}
