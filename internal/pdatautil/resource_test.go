// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestGroupByResourceLogs(t *testing.T) {
	testCases := []struct {
		name     string
		input    []resourceLogs
		expected []resourceLogs
	}{
		{
			name:     "empty",
			input:    []resourceLogs{},
			expected: []resourceLogs{},
		},
		{
			name: "single",
			input: []resourceLogs{newResourceLogs(1,
				newScopeLogs(1, 11, 12, 13),
			),
			},
			expected: []resourceLogs{newResourceLogs(1,
				newScopeLogs(1, 11, 12, 13),
			),
			},
		},
		{
			name: "distinct",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
					newScopeLogs(2, 21, 22, 23),
				),
				newResourceLogs(2,
					newScopeLogs(3, 31, 32, 33),
					newScopeLogs(4, 41, 42, 43),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
					newScopeLogs(2, 21, 22, 23),
				),
				newResourceLogs(2,
					newScopeLogs(3, 31, 32, 33),
					newScopeLogs(4, 41, 42, 43),
				),
			},
		},
		{
			name: "simple_merge_scopes",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
					newScopeLogs(1, 14, 15, 16),
				),
				newResourceLogs(2,
					newScopeLogs(2, 21, 22, 23),
					newScopeLogs(2, 24, 25, 26),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13, 14, 15, 16),
				),
				newResourceLogs(2,
					newScopeLogs(2, 21, 22, 23, 24, 25, 26),
				),
			},
		},
		{
			name: "merge_scopes_on_some_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
					newScopeLogs(1, 14, 15, 16),
				),
				newResourceLogs(2,
					newScopeLogs(2, 21, 22, 23),
					newScopeLogs(3, 31, 32, 33),
				),
				newResourceLogs(3,
					newScopeLogs(4, 41, 42, 43),
					newScopeLogs(4, 44, 45, 46),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13, 14, 15, 16),
				),
				newResourceLogs(2,
					newScopeLogs(2, 21, 22, 23),
					newScopeLogs(3, 31, 32, 33),
				),
				newResourceLogs(3,
					newScopeLogs(4, 41, 42, 43, 44, 45, 46),
				),
			},
		},
		{
			name: "leave_same_scopes_on_distinct_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
				),
				newResourceLogs(2,
					newScopeLogs(1, 11, 12, 13),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
				),
				newResourceLogs(2,
					newScopeLogs(1, 11, 12, 13),
				),
			},
		},
		{
			name: "merge_scopes_within_distinct_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
					newScopeLogs(1, 14, 15, 16),
				),
				newResourceLogs(2,
					newScopeLogs(1, 11, 12, 13),
					newScopeLogs(1, 14, 15, 16),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13, 14, 15, 16),
				),
				newResourceLogs(2,
					newScopeLogs(1, 11, 12, 13, 14, 15, 16),
				),
			},
		},
		{
			name: "merge_resources_preserve_distinct_scopes",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
					newScopeLogs(2, 21, 22, 23),
				),
				newResourceLogs(1,
					newScopeLogs(3, 31, 32, 33),
					newScopeLogs(4, 41, 42, 43),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
					newScopeLogs(2, 21, 22, 23),
					newScopeLogs(3, 31, 32, 33),
					newScopeLogs(4, 41, 42, 43),
				),
			},
		},
		{
			name: "merge_interleaved_scopes_within_resource",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
					newScopeLogs(2, 21, 22, 23),
					newScopeLogs(1, 14, 15, 16),
					newScopeLogs(2, 24, 25, 26),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13, 14, 15, 16),
					newScopeLogs(2, 21, 22, 23, 24, 25, 26),
				),
			},
		},
		{
			name: "merge_interleaved_scopes_across_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13),
					newScopeLogs(2, 21, 22, 23),
				),
				newResourceLogs(1,
					newScopeLogs(1, 14, 15, 16),
					newScopeLogs(2, 24, 25, 26),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 11, 12, 13, 14, 15, 16),
					newScopeLogs(2, 21, 22, 23, 24, 25, 26),
				),
			},
		},
		{
			name: "merge_interleaved_scopes_across_interleaved_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(22, 201, 202, 203),
				),
				newResourceLogs(2,
					newScopeLogs(33, 301, 302, 303),
					newScopeLogs(44, 401, 402, 403),
				),
				newResourceLogs(1,
					newScopeLogs(11, 104, 105, 106),
					newScopeLogs(22, 204, 205, 206),
				),
				newResourceLogs(2,
					newScopeLogs(33, 304, 305, 306),
					newScopeLogs(44, 404, 405, 406),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103, 104, 105, 106),
					newScopeLogs(22, 201, 202, 203, 204, 205, 206),
				),
				newResourceLogs(2,
					newScopeLogs(33, 301, 302, 303, 304, 305, 306),
					newScopeLogs(44, 401, 402, 403, 404, 405, 406),
				),
			},
		},
		{
			name: "merge_some_scopes_across_some_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(22, 201, 202, 203),
				),
				newResourceLogs(2,
					newScopeLogs(33, 301, 302, 303),
					newScopeLogs(11, 104, 105, 106),
				),
				newResourceLogs(1,
					newScopeLogs(33, 301, 302, 303),
					newScopeLogs(11, 104, 105, 106),
				),
				newResourceLogs(2,
					newScopeLogs(33, 304, 305, 306),
					newScopeLogs(44, 404, 405, 406),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103, 104, 105, 106),
					newScopeLogs(22, 201, 202, 203),
					newScopeLogs(33, 301, 302, 303),
				),
				newResourceLogs(2,
					newScopeLogs(33, 301, 302, 303, 304, 305, 306),
					newScopeLogs(11, 104, 105, 106),
					newScopeLogs(44, 404, 405, 406),
				),
			},
		},
		{
			name: "merge_all_resources_and_scopes",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 101, 102),
					newScopeLogs(1, 103, 104),
				),
				newResourceLogs(1,
					newScopeLogs(1, 105, 106),
					newScopeLogs(1, 107, 108),
				),
				newResourceLogs(1,
					newScopeLogs(1, 109, 110),
					newScopeLogs(1, 111, 112),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(1, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112),
				),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := plog.NewResourceLogsSlice()
			for _, r := range tc.input {
				r.setup(actual.AppendEmpty())
			}
			expected := plog.NewResourceLogsSlice()
			for _, r := range tc.expected {
				r.setup(expected.AppendEmpty())
			}

			GroupByResourceLogs(actual)
			assert.Equal(t, expected.Len(), actual.Len())
			for i := 0; i < expected.Len(); i++ {
				assert.NoError(t, plogtest.CompareResourceLogs(expected.At(i), actual.At(i)))
			}
		})
	}
}

type resourceLogs struct {
	num    int
	scopes []scopeLogs
}

func newResourceLogs(num int, scopes ...scopeLogs) resourceLogs {
	return resourceLogs{
		num:    num,
		scopes: scopes,
	}
}

func (r resourceLogs) setup(rl plog.ResourceLogs) {
	rl.Resource().Attributes().PutStr("attr.name", fmt.Sprintf("attr_%d", r.num))
	for _, s := range r.scopes {
		s.setup(rl.ScopeLogs().AppendEmpty())
	}
}
