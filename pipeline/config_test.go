// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pipeline

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/parser/json"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/transformer/copy"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func newDummyJSON(dummyID string) operator.Config {
	return operator.Config{Builder: json.NewJSONParserConfig(dummyID)}
}

func newDummyCopy(dummyID string) operator.Config {
	return operator.Config{Builder: copy.NewCopyOperatorConfig(dummyID)}
}

type deduplicateTestCase struct {
	name        string
	ops         func() Config
	expectedOps Config
}

func TestDeduplicateIDs(t *testing.T) {
	cases := []deduplicateTestCase{
		{
			"one_op_rename",
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser1"))
				return ops
			}(),
		},
		{
			"multi_op_rename",
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))

				return ops
			},
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser1"))
				ops = append(ops, newDummyJSON("json_parser2"))
				ops = append(ops, newDummyJSON("json_parser3"))
				ops = append(ops, newDummyJSON("json_parser4"))
				return ops
			}(),
		},
		{
			"different_ops",
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyCopy("copy"))
				ops = append(ops, newDummyCopy("copy"))

				return ops
			},
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser1"))
				ops = append(ops, newDummyJSON("json_parser2"))
				ops = append(ops, newDummyCopy("copy"))
				ops = append(ops, newDummyCopy("copy1"))
				return ops
			}(),
		},
		{
			"unordered",
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyCopy("copy"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyCopy("copy"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyCopy("copy"))
				ops = append(ops, newDummyJSON("json_parser1"))
				ops = append(ops, newDummyCopy("copy1"))
				ops = append(ops, newDummyJSON("json_parser2"))
				return ops
			}(),
		},
		{
			"already_renamed",
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser3"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser1"))
				ops = append(ops, newDummyJSON("json_parser2"))
				ops = append(ops, newDummyJSON("json_parser3"))
				ops = append(ops, newDummyJSON("json_parser4"))
				return ops
			}(),
		},
		{
			"iterate_twice",
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser3"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser1"))
				ops = append(ops, newDummyJSON("json_parser3"))
				ops = append(ops, newDummyJSON("json_parser2"))
				ops = append(ops, newDummyJSON("json_parser4"))
				return ops
			}(),
		},
	}

	for _, tc := range cases {
		t.Run("Deduplicate/"+tc.name, func(t *testing.T) {
			ops := tc.ops()
			ops.dedeplucateIDs()
			require.Equal(t, ops, tc.expectedOps)
		})
	}
}

func TestUpdateOutputIDs(t *testing.T) {
	cases := []struct {
		defaultOut operator.Operator
		ops        func() Config
		outMap     map[string][]string
		name       string
	}{
		{
			name: "one_op_rename",
			ops: func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			outMap: map[string][]string{
				"json_parser":  {"json_parser1"},
				"json_parser1": nil,
			},
			defaultOut: nil,
		},
		{
			name: "multi_op_rename",
			ops: func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			outMap: map[string][]string{
				"json_parser":  {"json_parser1"},
				"json_parser1": {"json_parser2"},
				"json_parser2": {"json_parser3"},
				"json_parser3": nil,
			},
			defaultOut: nil,
		},
		{
			name: "different_ops",
			ops: func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyCopy("copy"))
				ops = append(ops, newDummyCopy("copy"))
				return ops
			},
			outMap: map[string][]string{
				"json_parser":  {"json_parser1"},
				"json_parser1": {"json_parser2"},
				"json_parser2": {"copy"},
				"copy":         {"copy1"},
				"copy1":        nil,
			},
			defaultOut: nil,
		},
		{
			name: "unordered",
			ops: func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyCopy("copy"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyCopy("copy"))
				return ops
			},
			outMap: map[string][]string{
				"json_parser":  {"copy"},
				"copy":         {"json_parser1"},
				"json_parser1": {"copy1"},
				"copy1":        nil,
			},
			defaultOut: nil,
		},
		{
			name: "already_renamed",
			ops: func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser3"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			outMap: map[string][]string{
				"json_parser":  {"json_parser1"},
				"json_parser1": {"json_parser2"},
				"json_parser2": {"json_parser3"},
				"json_parser3": {"json_parser4"},
				"json_parser4": nil,
			},
			defaultOut: nil,
		},
		{
			name: "one_op_rename",
			ops: func() Config {
				var ops Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			outMap: map[string][]string{
				"json_parser":  {"json_parser1"},
				"json_parser1": {"fake"},
			},
			defaultOut: testutil.NewFakeOutput(t),
		},
	}

	for _, tc := range cases {
		t.Run("UpdateOutputIDs/"+tc.name, func(t *testing.T) {
			pipeline, err := tc.ops().Build(testutil.Logger(t), tc.defaultOut)
			require.NoError(t, err)
			ops := pipeline.Operators()

			expectedNumOps := len(tc.outMap)
			if tc.defaultOut != nil {
				expectedNumOps++
			}
			require.Equal(t, expectedNumOps, len(ops))

			for i := 0; i < len(ops); i++ {
				id := ops[i].ID()
				if id == "fake" {
					require.Nil(t, ops[i].GetOutputIDs())
					continue
				}
				expectedOuts, ok := tc.outMap[id]
				require.True(t, ok)
				actualOuts := ops[i].GetOutputIDs()
				sort.Strings(expectedOuts)
				sort.Strings(actualOuts)
				require.Equal(t, expectedOuts, actualOuts)
			}
		})
	}
}
