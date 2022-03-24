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

package restructure

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestRestructureOperator(t *testing.T) {
	os.Setenv("TEST_RESTRUCTURE_OPERATOR_ENV", "foo")
	defer os.Unsetenv("TEST_RESTRUCTURE_OPERATOR_ENV")

	newTestEntry := func() *entry.Entry {
		e := entry.New()
		e.Timestamp = time.Unix(1586632809, 0)
		e.Body = map[string]interface{}{
			"key": "val",
			"nested": map[string]interface{}{
				"nestedkey": "nestedval",
			},
		}
		return e
	}

	cases := []struct {
		name   string
		ops    []Op
		input  *entry.Entry
		output *entry.Entry
	}{
		{
			name:   "Nothing",
			input:  newTestEntry(),
			output: newTestEntry(),
		},
		{
			name: "AddValue",
			ops: []Op{
				{
					&OpAdd{
						Field: entry.NewBodyField("new"),
						Value: "message",
					},
				},
			},
			input: newTestEntry(),
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body.(map[string]interface{})["new"] = "message"
				return e
			}(),
		},
		{
			name: "AddValueExpr",
			ops: []Op{
				{
					&OpAdd{
						Field: entry.NewBodyField("new"),
						program: func() *vm.Program {
							vm, err := expr.Compile(`body.key + "_suffix"`)
							require.NoError(t, err)
							return vm
						}(),
					},
				},
			},
			input: newTestEntry(),
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body.(map[string]interface{})["new"] = "val_suffix"
				return e
			}(),
		},
		{
			name: "AddValueExprEnv",
			ops: []Op{
				{
					&OpAdd{
						Field: entry.NewBodyField("new"),
						program: func() *vm.Program {
							vm, err := expr.Compile(`env("TEST_RESTRUCTURE_OPERATOR_ENV")`)
							require.NoError(t, err)
							return vm
						}(),
					},
				},
			},
			input: newTestEntry(),
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body.(map[string]interface{})["new"] = "foo"
				return e
			}(),
		},
		{
			name: "Remove",
			ops: []Op{
				{
					&OpRemove{entry.NewBodyField("nested")},
				},
			},
			input: newTestEntry(),
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
				}
				return e
			}(),
		},
		{
			name: "Retain",
			ops: []Op{
				{
					&OpRetain{[]entry.Field{entry.NewBodyField("key")}},
				},
			},
			input: newTestEntry(),
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
				}
				return e
			}(),
		},
		{
			name: "Move",
			ops: []Op{
				{
					&OpMove{
						From: entry.NewBodyField("key"),
						To:   entry.NewBodyField("newkey"),
					},
				},
			},
			input: newTestEntry(),
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"newkey": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				return e
			}(),
		},
		{
			name: "Flatten",
			ops: []Op{
				{
					&OpFlatten{
						Field: entry.BodyField{
							Keys: []string{"nested"},
						},
					},
				},
			},
			input: newTestEntry(),
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key":       "val",
					"nestedkey": "nestedval",
				}
				return e
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewRestructureOperatorConfig("test")
			cfg.OutputIDs = []string{"fake"}
			op, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			restructure := op.(*RestructureOperator)
			fake := testutil.NewFakeOutput(t)
			restructure.SetOutputs([]operator.Operator{fake})
			restructure.ops = tc.ops

			err = restructure.Process(context.Background(), tc.input)
			require.NoError(t, err)

			fake.ExpectEntry(t, tc.output)
		})
	}
}

func TestRestructureSerializeRoundtrip(t *testing.T) {
	cases := []struct {
		name string
		op   Op
	}{
		{
			name: "AddValue",
			op: Op{&OpAdd{
				Field: entry.NewBodyField("new"),
				Value: "message",
			}},
		},
		{
			name: "AddValueExpr",
			op: Op{&OpAdd{
				Field: entry.NewBodyField("new"),
				ValueExpr: func() *string {
					s := `body.key + "_suffix"`
					return &s
				}(),
				program: func() *vm.Program {
					vm, err := expr.Compile(`body.key + "_suffix"`)
					require.NoError(t, err)
					return vm
				}(),
			}},
		},
		{
			name: "Remove",
			op:   Op{&OpRemove{entry.NewBodyField("nested")}},
		},
		{
			name: "Retain",
			op:   Op{&OpRetain{[]entry.Field{entry.NewBodyField("key")}}},
		},
		{
			name: "Move",
			op: Op{&OpMove{
				From: entry.NewBodyField("key"),
				To:   entry.NewBodyField("newkey"),
			}},
		},
		{
			name: "Flatten",
			op: Op{&OpFlatten{
				Field: entry.BodyField{
					Keys: []string{"body", "nested"},
				},
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tc.op)
			require.NoError(t, err)

			var jsonOp Op
			err = json.Unmarshal(jsonBytes, &jsonOp)
			require.NoError(t, err)

			require.Equal(t, tc.op, jsonOp)

			yamlBytes, err := yaml.Marshal(tc.op)
			require.NoError(t, err)

			var yamlOp Op
			err = yaml.UnmarshalStrict(yamlBytes, &yamlOp)
			require.NoError(t, err)

			require.Equal(t, tc.op, yamlOp)
		})
	}
}

func TestUnmarshalAll(t *testing.T) {
	configYAML := `
type: restructure
id: my_restructure
output: test_output
ops:
  - add:
      field: "body.message"
      value: "val"
  - add:
      field: "body.message_suffix"
      value_expr: "body.message + \"_suffix\""
  - remove: "body.message"
  - retain:
      - "body.message_retain"
  - flatten: "body.message_flatten"
  - move:
      from: "body.message1"
      to: "body.message2"
`

	configJSON := `
{
  "type": "restructure",
  "id": "my_restructure",
  "output": "test_output",
  "ops": [{
    "add": {
      "field": "body.message",
      "value": "val"
    }
  },{
    "add": {
      "field": "body.message_suffix",
      "value_expr": "body.message + \"_suffix\""
    }
  },{
    "remove": "body.message"
  },{
    "retain": [
      "body.message_retain"
    ]
  },{
    "flatten": "body.message_flatten"
  },{
    "move": {
      "from": "body.message1",
      "to": "body.message2"
    }
  }]
}`

	expected := operator.Config{
		Builder: &RestructureOperatorConfig{
			TransformerConfig: helper.TransformerConfig{
				WriterConfig: helper.WriterConfig{
					BasicConfig: helper.BasicConfig{
						OperatorID:   "my_restructure",
						OperatorType: "restructure",
					},
					OutputIDs: []string{"test_output"},
				},
				OnError: helper.SendOnError,
			},
			Ops: []Op{
				{&OpAdd{
					Field: entry.NewBodyField("message"),
					Value: "val",
				}},
				{&OpAdd{
					Field: entry.NewBodyField("message_suffix"),
					ValueExpr: func() *string {
						s := `body.message + "_suffix"`
						return &s
					}(),
					program: func() *vm.Program {
						vm, err := expr.Compile(`body.message + "_suffix"`)
						require.NoError(t, err)
						return vm
					}(),
				}},
				{&OpRemove{
					Field: entry.NewBodyField("message"),
				}},
				{&OpRetain{
					Fields: []entry.Field{
						entry.NewBodyField("message_retain"),
					},
				}},
				{&OpFlatten{
					Field: entry.BodyField{
						Keys: []string{"message_flatten"},
					},
				}},
				{&OpMove{
					From: entry.NewBodyField("message1"),
					To:   entry.NewBodyField("message2"),
				}},
			},
		},
	}

	var unmarshalledYAML operator.Config
	err := yaml.UnmarshalStrict([]byte(configYAML), &unmarshalledYAML)
	require.NoError(t, err)
	require.Equal(t, expected, unmarshalledYAML)

	var unmarshalledJSON operator.Config
	err = json.Unmarshal([]byte(configJSON), &unmarshalledJSON)
	require.NoError(t, err)
	require.Equal(t, expected, unmarshalledJSON)
}

func TestOpType(t *testing.T) {
	cases := []struct {
		op           OpApplier
		expectedType string
	}{
		{
			&OpAdd{},
			"add",
		},
		{
			&OpRemove{},
			"remove",
		},
		{
			&OpRetain{},
			"retain",
		},
		{
			&OpMove{},
			"move",
		},
		{
			&OpFlatten{},
			"flatten",
		},
	}

	for _, tc := range cases {
		t.Run(tc.expectedType, func(t *testing.T) {
			require.Equal(t, tc.expectedType, tc.op.Type())
		})
	}

	t.Run("InvalidOpType", func(t *testing.T) {
		raw := "- unknown: test"
		var ops []Op
		err := yaml.UnmarshalStrict([]byte(raw), &ops)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown op type")
	})
}
