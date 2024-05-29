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
			actual := plog.NewScopeLogsSlice()
			for _, s := range tc.input {
				s.setup(actual.AppendEmpty())
			}
			expected := plog.NewScopeLogsSlice()
			for _, s := range tc.expected {
				s.setup(expected.AppendEmpty())
			}

			GroupByScopeLogs(actual)
			assert.Equal(t, expected.Len(), actual.Len())
			for i := 0; i < expected.Len(); i++ {
				assert.NoError(t, plogtest.CompareScopeLogs(expected.At(i), actual.At(i)))
			}
		})
	}
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
