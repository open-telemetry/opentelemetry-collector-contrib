// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/json"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/copy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/noop"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestBuildPipelineSuccess(t *testing.T) {
	cfg := Config{
		Operators: []operator.Config{
			{
				Builder: noop.NewConfigWithID("noop"),
			},
		},
	}

	pipe, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.Equal(t, 1, len(pipe.Operators()))
}

func TestBuildPipelineNoLogger(t *testing.T) {
	cfg := Config{
		Operators: []operator.Config{
			{
				Builder: noop.NewConfigWithID("noop"),
			},
		},
	}

	pipe, err := cfg.Build(nil)
	require.EqualError(t, err, "logger must be provided")
	require.Nil(t, pipe)
}

func TestBuildPipelineNilOperators(t *testing.T) {
	cfg := Config{}

	pipe, err := cfg.Build(testutil.Logger(t))
	require.EqualError(t, err, "operators must be specified")
	require.Nil(t, pipe)
}

func TestBuildPipelineEmptyOperators(t *testing.T) {
	cfg := Config{
		Operators: []operator.Config{},
	}

	pipe, err := cfg.Build(testutil.Logger(t))
	require.EqualError(t, err, "empty pipeline not allowed")
	require.Nil(t, pipe)
}

func TestBuildAPipelineDefaultOperator(t *testing.T) {
	cfg := Config{
		Operators: []operator.Config{
			{
				Builder: noop.NewConfigWithID("noop"),
			},
			{
				Builder: noop.NewConfigWithID("noop1"),
			},
		},
		DefaultOutput: testutil.NewFakeOutput(t),
	}

	pipe, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	ops := pipe.Operators()
	require.Equal(t, 3, len(ops))

	exists := make(map[string]bool)

	for _, op := range ops {
		switch op.ID() {
		case "noop":
			require.Equal(t, 1, len(op.GetOutputIDs()))
			require.Equal(t, "noop1", op.GetOutputIDs()[0])
			exists["noop"] = true
		case "noop1":
			require.Equal(t, 1, len(op.GetOutputIDs()))
			require.Equal(t, "fake", op.GetOutputIDs()[0])
			exists["noop1"] = true
		case "fake":
			require.Equal(t, 0, len(op.GetOutputIDs()))
			exists["fake"] = true
		}
	}
	require.True(t, exists["noop"])
	require.True(t, exists["noop1"])
	require.True(t, exists["fake"])
}

func TestDeduplicateIDs(t *testing.T) {
	cases := []struct {
		name        string
		ops         func() []operator.Config
		expectedOps []operator.Config
	}{
		{
			"one_op_rename",
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser1"))
				return ops
			}(),
		},
		{
			"multi_op_rename",
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))

				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
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
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyCopy("copy"))
				ops = append(ops, newDummyCopy("copy"))
				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
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
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyCopy("copy"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyCopy("copy"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
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
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser3"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
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
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser3"))
				ops = append(ops, newDummyJSON("json_parser"))
				ops = append(ops, newDummyJSON("json_parser"))
				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
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
			dedeplucateIDs(ops)
			require.Equal(t, ops, tc.expectedOps)
		})
	}
}

func TestUpdateOutputIDs(t *testing.T) {
	cases := []struct {
		defaultOut operator.Operator
		ops        func() []operator.Config
		outMap     map[string][]string
		name       string
	}{
		{
			name: "one_op_rename",
			ops: func() []operator.Config {
				var ops []operator.Config
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
			ops: func() []operator.Config {
				var ops []operator.Config
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
			ops: func() []operator.Config {
				var ops []operator.Config
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
			ops: func() []operator.Config {
				var ops []operator.Config
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
			ops: func() []operator.Config {
				var ops []operator.Config
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
			ops: func() []operator.Config {
				var ops []operator.Config
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
			pipeline, err := Config{
				Operators:     tc.ops(),
				DefaultOutput: tc.defaultOut,
			}.Build(testutil.Logger(t))
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

func newDummyJSON(dummyID string) operator.Config {
	return operator.Config{Builder: json.NewConfigWithID(dummyID)}
}

func newDummyCopy(dummyID string) operator.Config {
	return operator.Config{Builder: copy.NewConfigWithID(dummyID)}
}
