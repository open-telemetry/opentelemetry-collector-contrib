// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestFlattenResourceLogs(t *testing.T) {
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
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 111),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 111),
				),
			},
		},
		{
			name: "flatten_single_scope_in_single_resource",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1, newScopeLogs(11, 101)),
				newResourceLogs(1, newScopeLogs(11, 102)),
				newResourceLogs(1, newScopeLogs(11, 103)),
			},
		},
		{
			name: "flatten_multiple_scopes_in_single_resource",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(22, 201, 202, 203),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1, newScopeLogs(11, 101)),
				newResourceLogs(1, newScopeLogs(11, 102)),
				newResourceLogs(1, newScopeLogs(11, 103)),
				newResourceLogs(1, newScopeLogs(22, 201)),
				newResourceLogs(1, newScopeLogs(22, 202)),
				newResourceLogs(1, newScopeLogs(22, 203)),
			},
		},
		{
			name: "flatten_single_scope_in_multiple_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
				),
				newResourceLogs(2,
					newScopeLogs(11, 104, 105, 106),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1, newScopeLogs(11, 101)),
				newResourceLogs(1, newScopeLogs(11, 102)),
				newResourceLogs(1, newScopeLogs(11, 103)),
				newResourceLogs(2, newScopeLogs(11, 104)),
				newResourceLogs(2, newScopeLogs(11, 105)),
				newResourceLogs(2, newScopeLogs(11, 106)),
			},
		},
		{
			name: "flatten_multiple_scopes_in_multiple_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(22, 201, 202, 203),
				),
				newResourceLogs(2,
					newScopeLogs(11, 104, 105, 106),
					newScopeLogs(22, 204, 205, 206),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1, newScopeLogs(11, 101)),
				newResourceLogs(1, newScopeLogs(11, 102)),
				newResourceLogs(1, newScopeLogs(11, 103)),
				newResourceLogs(1, newScopeLogs(22, 201)),
				newResourceLogs(1, newScopeLogs(22, 202)),
				newResourceLogs(1, newScopeLogs(22, 203)),
				newResourceLogs(2, newScopeLogs(11, 104)),
				newResourceLogs(2, newScopeLogs(11, 105)),
				newResourceLogs(2, newScopeLogs(11, 106)),
				newResourceLogs(2, newScopeLogs(22, 204)),
				newResourceLogs(2, newScopeLogs(22, 205)),
				newResourceLogs(2, newScopeLogs(22, 206)),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := setupResourceLogsSlice(tc.input)
			expected := setupResourceLogsSlice(tc.expected)
			FlattenLogs(actual)
			assert.Equal(t, expected.Len(), actual.Len())
			for i := 0; i < expected.Len(); i++ {
				assert.NoError(t, plogtest.CompareResourceLogs(expected.At(i), actual.At(i)))
			}
		})
	}
}

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
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
				),
			},
		},
		{
			name: "distinct",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(22, 201, 202, 203),
				),
				newResourceLogs(2,
					newScopeLogs(33, 301, 302, 303),
					newScopeLogs(44, 401, 402, 403),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(22, 201, 202, 203),
				),
				newResourceLogs(2,
					newScopeLogs(33, 301, 302, 303),
					newScopeLogs(44, 401, 402, 403),
				),
			},
		},
		{
			name: "simple_merge_scopes",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(11, 104, 105, 106),
				),
				newResourceLogs(2,
					newScopeLogs(22, 201, 202, 203),
					newScopeLogs(22, 204, 205, 206),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103, 104, 105, 106),
				),
				newResourceLogs(2,
					newScopeLogs(22, 201, 202, 203, 204, 205, 206),
				),
			},
		},
		{
			name: "merge_scopes_on_some_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(11, 104, 105, 106),
				),
				newResourceLogs(2,
					newScopeLogs(22, 201, 202, 203),
					newScopeLogs(33, 301, 302, 303),
				),
				newResourceLogs(3,
					newScopeLogs(44, 401, 402, 403),
					newScopeLogs(44, 404, 405, 406),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103, 104, 105, 106),
				),
				newResourceLogs(2,
					newScopeLogs(22, 201, 202, 203),
					newScopeLogs(33, 301, 302, 303),
				),
				newResourceLogs(3,
					newScopeLogs(44, 401, 402, 403, 404, 405, 406),
				),
			},
		},
		{
			name: "leave_same_scopes_on_distinct_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
				),
				newResourceLogs(2,
					newScopeLogs(11, 101, 102, 103),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
				),
				newResourceLogs(2,
					newScopeLogs(11, 101, 102, 103),
				),
			},
		},
		{
			name: "merge_scopes_within_distinct_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(11, 104, 105, 106),
				),
				newResourceLogs(2,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(11, 104, 105, 106),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103, 104, 105, 106),
				),
				newResourceLogs(2,
					newScopeLogs(11, 101, 102, 103, 104, 105, 106),
				),
			},
		},
		{
			name: "merge_resources_preserve_distinct_scopes",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(22, 201, 202, 203),
				),
				newResourceLogs(1,
					newScopeLogs(33, 301, 302, 303),
					newScopeLogs(44, 401, 402, 403),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(22, 201, 202, 203),
					newScopeLogs(33, 301, 302, 303),
					newScopeLogs(44, 401, 402, 403),
				),
			},
		},
		{
			name: "merge_interleaved_scopes_within_resource",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(22, 201, 202, 203),
					newScopeLogs(11, 104, 105, 106),
					newScopeLogs(22, 204, 205, 206),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103, 104, 105, 106),
					newScopeLogs(22, 201, 202, 203, 204, 205, 206),
				),
			},
		},
		{
			name: "merge_interleaved_scopes_across_resources",
			input: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103),
					newScopeLogs(22, 201, 202, 203),
				),
				newResourceLogs(1,
					newScopeLogs(11, 104, 105, 106),
					newScopeLogs(22, 204, 205, 206),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103, 104, 105, 106),
					newScopeLogs(22, 201, 202, 203, 204, 205, 206),
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
					newScopeLogs(11, 101, 102),
					newScopeLogs(11, 103, 104),
				),
				newResourceLogs(1,
					newScopeLogs(11, 105, 106),
					newScopeLogs(11, 107, 108),
				),
				newResourceLogs(1,
					newScopeLogs(11, 109, 110),
					newScopeLogs(11, 111, 112),
				),
			},
			expected: []resourceLogs{
				newResourceLogs(1,
					newScopeLogs(11, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112),
				),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := setupResourceLogsSlice(tc.input)
			expected := setupResourceLogsSlice(tc.expected)
			GroupByResourceLogs(actual)
			assert.Equal(t, expected.Len(), actual.Len())
			for i := 0; i < expected.Len(); i++ {
				assert.NoError(t, plogtest.CompareResourceLogs(expected.At(i), actual.At(i)))
			}
		})
	}
}

func TestGroupByScopeLogs(t *testing.T) {
	testCases := []struct {
		name     string
		input    []scopeLogs
		expected []scopeLogs
	}{
		{
			name:     "empty",
			input:    []scopeLogs{},
			expected: []scopeLogs{},
		},
		{
			name: "single",
			input: []scopeLogs{
				newScopeLogs(1, 11, 12, 13),
			},
			expected: []scopeLogs{
				newScopeLogs(1, 11, 12, 13),
			},
		},
		{
			name: "distinct",
			input: []scopeLogs{
				newScopeLogs(1, 11, 12, 13),
				newScopeLogs(2, 21, 22, 23),
			},
			expected: []scopeLogs{
				newScopeLogs(1, 11, 12, 13),
				newScopeLogs(2, 21, 22, 23),
			},
		},
		{
			name: "simple_merge",
			input: []scopeLogs{
				newScopeLogs(1, 11, 12, 13),
				newScopeLogs(1, 14, 15, 16),
			},
			expected: []scopeLogs{
				newScopeLogs(1, 11, 12, 13, 14, 15, 16),
			},
		},
		{
			name: "interleaved",
			input: []scopeLogs{
				newScopeLogs(1, 11, 12, 13),
				newScopeLogs(2, 21, 22, 23),
				newScopeLogs(1, 14, 15, 16),
				newScopeLogs(2, 24, 25, 26),
			},
			expected: []scopeLogs{
				newScopeLogs(1, 11, 12, 13, 14, 15, 16),
				newScopeLogs(2, 21, 22, 23, 24, 25, 26),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := setupScopeLogsSlice(tc.input)
			expected := setupScopeLogsSlice(tc.expected)
			GroupByScopeLogs(actual)
			assert.Equal(t, expected.Len(), actual.Len())
			for i := 0; i < expected.Len(); i++ {
				assert.NoError(t, plogtest.CompareScopeLogs(expected.At(i), actual.At(i)))
			}
		})
	}
}

func TestHashScopeLogs(t *testing.T) {
	schemas := []string{"", "schema_1", "schema_2"}
	names := []string{"", "name_1", "name_2"}
	versions := []string{"", "version_1", "version_2"}
	attributes := []map[string]any{
		{},
		{"attr.name": "attr_1"},
		{"attr.name": "attr_2"},
		{"attr.name": "attr_1", "other.name": "other"},
	}

	distinctScopeLogs := make([]plog.ScopeLogs, 0, len(schemas)*len(names)*len(versions)*len(attributes))

	for _, schema := range schemas {
		for _, name := range names {
			for _, version := range versions {
				for _, attr := range attributes {
					sl := plog.NewScopeLogs()
					sl.SetSchemaUrl(schema)
					ss := sl.Scope()
					ss.SetName(name)
					ss.SetVersion(version)
					require.NoError(t, ss.Attributes().FromRaw(attr))
					distinctScopeLogs = append(distinctScopeLogs, sl)
				}
			}
		}
	}

	for i, slOne := range distinctScopeLogs {
		for j, slTwo := range distinctScopeLogs {
			if i == j {
				assert.Equal(t, HashScopeLogs(slOne), HashScopeLogs(slTwo))
			} else {
				assert.NotEqual(t, HashScopeLogs(slOne), HashScopeLogs(slTwo))
			}
		}
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

func setupResourceLogsSlice(trls []resourceLogs) plog.ResourceLogsSlice {
	rls := plog.NewResourceLogsSlice()
	for _, trl := range trls {
		trl.setup(rls.AppendEmpty())
	}
	return rls
}

type scopeLogs struct {
	num        int
	recordNums []int
}

func newScopeLogs(num int, recordNums ...int) scopeLogs {
	return scopeLogs{
		num:        num,
		recordNums: recordNums,
	}
}

func (s scopeLogs) setup(sl plog.ScopeLogs) {
	sl.SetSchemaUrl(fmt.Sprintf("schema_%d", s.num))
	ss := sl.Scope()
	ss.SetName(fmt.Sprintf("name_%d", s.num))
	ss.SetVersion(fmt.Sprintf("version_%d", s.num))
	ss.Attributes().PutStr("attr.name", fmt.Sprintf("attr_%d", s.num))
	for _, n := range s.recordNums {
		lr := sl.LogRecords().AppendEmpty()
		lr.Attributes().PutInt("num", int64(n))
		lr.Body().SetInt(int64(n))
	}
}

func setupScopeLogsSlice(tsls []scopeLogs) plog.ScopeLogsSlice {
	sls := plog.NewScopeLogsSlice()
	for _, tsl := range tsls {
		tsl.setup(sls.AppendEmpty())
	}
	return sls
}
