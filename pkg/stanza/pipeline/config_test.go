// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/jsonparser"
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

	set := componenttest.NewNopTelemetrySettings()
	pipe, err := cfg.Build(set)
	require.NoError(t, err)
	require.Len(t, pipe.Operators(), 1)
}

func TestBuildPipelineNoLogger(t *testing.T) {
	cfg := Config{
		Operators: []operator.Config{
			{
				Builder: noop.NewConfigWithID("noop"),
			},
		},
	}

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = nil
	pipe, err := cfg.Build(set)
	require.EqualError(t, err, "logger must be provided")
	require.Nil(t, pipe)
}

func TestBuildPipelineNilOperators(t *testing.T) {
	cfg := Config{}

	set := componenttest.NewNopTelemetrySettings()
	pipe, err := cfg.Build(set)
	require.EqualError(t, err, "operators must be specified")
	require.Nil(t, pipe)
}

func TestBuildPipelineEmptyOperators(t *testing.T) {
	cfg := Config{
		Operators: []operator.Config{},
	}

	set := componenttest.NewNopTelemetrySettings()
	pipe, err := cfg.Build(set)
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

	set := componenttest.NewNopTelemetrySettings()
	pipe, err := cfg.Build(set)
	require.NoError(t, err)

	ops := pipe.Operators()
	require.Len(t, ops, 3)

	exists := make(map[string]bool)

	for _, op := range ops {
		switch op.ID() {
		case "noop":
			require.Len(t, op.GetOutputIDs(), 1)
			require.Equal(t, "noop1", op.GetOutputIDs()[0])
			exists["noop"] = true
		case "noop1":
			require.Len(t, op.GetOutputIDs(), 1)
			require.Equal(t, "fake", op.GetOutputIDs()[0])
			exists["noop1"] = true
		case "fake":
			require.Empty(t, op.GetOutputIDs())
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
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"))
				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser1"))
				return ops
			}(),
		},
		{
			"multi_op_rename",
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"))

				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser1"),
					newDummyJSON("json_parser2"),
					newDummyJSON("json_parser3"),
					newDummyJSON("json_parser4"))
				return ops
			}(),
		},
		{
			"different_ops",
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyCopy("copy"),
					newDummyCopy("copy"))
				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser1"),
					newDummyJSON("json_parser2"),
					newDummyCopy("copy"),
					newDummyCopy("copy1"))
				return ops
			}(),
		},
		{
			"unordered",
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyCopy("copy"),
					newDummyJSON("json_parser"),
					newDummyCopy("copy"),
					newDummyJSON("json_parser"))
				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyCopy("copy"),
					newDummyJSON("json_parser1"),
					newDummyCopy("copy1"),
					newDummyJSON("json_parser2"))
				return ops
			}(),
		},
		{
			"already_renamed",
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser3"),
					newDummyJSON("json_parser"))
				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser1"),
					newDummyJSON("json_parser2"),
					newDummyJSON("json_parser3"),
					newDummyJSON("json_parser4"))
				return ops
			}(),
		},
		{
			"iterate_twice",
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser3"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"))
				return ops
			},
			func() []operator.Config {
				var ops []operator.Config
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser1"),
					newDummyJSON("json_parser3"),
					newDummyJSON("json_parser2"),
					newDummyJSON("json_parser4"))
				return ops
			}(),
		},
	}

	for _, tc := range cases {
		t.Run("Deduplicate/"+tc.name, func(t *testing.T) {
			ops := tc.ops()
			dedeplucateIDs(ops)
			require.Equal(t, tc.expectedOps, ops)
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
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"))
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
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"))
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
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyCopy("copy"),
					newDummyCopy("copy"))
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
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyCopy("copy"),
					newDummyJSON("json_parser"),
					newDummyCopy("copy"))
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
				ops = append(ops,
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser"),
					newDummyJSON("json_parser3"),
					newDummyJSON("json_parser"))
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
				ops = append(ops, newDummyJSON("json_parser"), newDummyJSON("json_parser"))
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
			set := componenttest.NewNopTelemetrySettings()
			pipeline, err := Config{
				Operators:     tc.ops(),
				DefaultOutput: tc.defaultOut,
			}.Build(set)
			require.NoError(t, err)
			ops := pipeline.Operators()

			expectedNumOps := len(tc.outMap)
			if tc.defaultOut != nil {
				expectedNumOps++
			}
			require.Len(t, ops, expectedNumOps)

			for i := range ops {
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
	return operator.Config{Builder: jsonparser.NewConfigWithID(dummyID)}
}

func newDummyCopy(dummyID string) operator.Config {
	copyConfig := copy.NewConfigWithID(dummyID)
	copyConfig.From = entry.NewBodyField("body.something")
	copyConfig.To = entry.NewBodyField("body.something_else")
	return operator.Config{
		Builder: copyConfig,
	}
}
