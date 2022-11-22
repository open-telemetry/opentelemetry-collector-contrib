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

package pipeline // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/add"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/copy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/filter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/flatten"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/move"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/noop"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/recombine"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/remove"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/retain"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

// buildPipeline returns fake output and pipeline with multiple operators
func buildPipeline(t *testing.T) (*testutil.FakeOutput, *pipeline.DirectedPipeline) {
	opAdd := add.NewConfigWithID("add")
	opAdd.Value = "test"
	opAdd.Field = entry.NewResourceField("new_field")

	opCopy := operator.NewConfig(copy.NewConfigWithID("copy"))
	err := opCopy.UnmarshalJSON([]byte(`{"from": "resource.new_field", "to": "resource.field_copy", "type": "copy"}`))
	require.NoError(t, err)

	opFilter := filter.NewConfigWithID("filter")
	opFilter.Expression = "body.filter matches 'true'"

	opFlatten := operator.NewConfig(flatten.NewConfigWithID("flatten"))
	err = opFlatten.UnmarshalJSON([]byte(`{"type": "flatten", "field": "body.to_flatten"}`))
	require.NoError(t, err)

	opMove := operator.NewConfig(move.NewConfigWithID("move"))
	err = opMove.UnmarshalJSON([]byte(`{"type": "move", "from": "resource.field_copy", "to": "resource.field_copy_moved"}`))
	require.NoError(t, err)

	opNoop := operator.NewConfig(noop.NewConfigWithID("noop"))
	err = opNoop.UnmarshalJSON([]byte(`{"type": "noop"}`))
	require.NoError(t, err)

	opRecombine := recombine.NewConfigWithID("recombine")
	opRecombine.IsLastEntry = "body.recombine matches \"true\""
	opRecombine.CombineField = entry.NewBodyField("body")

	opRemove := operator.NewConfig(remove.NewConfigWithID("remove"))
	err = opRemove.UnmarshalJSON([]byte(`{"type": "remove", "field": "body.to_remove"}`))
	require.NoError(t, err)

	opRetain := operator.NewConfig(retain.NewConfigWithID("retain"))
	err = opRetain.UnmarshalJSON([]byte(`{"type": "retain", "fields": ["body"]}`))
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)

	cfg := pipeline.Config{
		Operators: []operator.Config{
			{
				Builder: opAdd,
			},
			opCopy,
			{
				Builder: opFilter,
			},
			opFlatten,
			opMove,
			opNoop,
			{
				Builder: opRecombine,
			},
			opRemove,
			opRetain,
		},
		DefaultOutput: fake,
	}

	pipeline, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	return fake, pipeline
}

func TestMe(t *testing.T) {
	fake, pipeline := buildPipeline(t)

	testCases := []struct {
		Name      string
		Bodies    []map[string]interface{}
		Expected  map[string]interface{}
		Processed int
	}{
		{
			Name: "Basic",
			Bodies: []map[string]interface{}{
				{
					"to_flatten": map[string]interface{}{
						"field1": "value1",
					},
					"filter":    "false",
					"to_remove": "remove_me",
					"recombine": "true",
					"body":      "This is my body",
				},
			},
			Expected: map[string]interface{}{
				"body":      "This is my body",
				"filter":    "false",
				"recombine": "true",
				"field1":    "value1",
			},
			Processed: 1,
		},
		{
			Name: "Filter out",
			Bodies: []map[string]interface{}{
				{
					"to_flatten": map[string]interface{}{
						"field1": "value1",
					},
					"filter":    "true",
					"to_remove": "remove_me",
					"recombine": "true",
					"body":      "This is my body",
				},
			},
			Expected:  map[string]interface{}{},
			Processed: 0,
		},
		{
			Name: "Recombine",
			Bodies: []map[string]interface{}{
				{
					"to_flatten": map[string]interface{}{
						"field1": "value1",
					},
					"filter":    "false",
					"to_remove": "remove_me",
					"recombine": "false",
					"body":      "This is first line",
				},
				{
					"to_flatten": map[string]interface{}{
						"field1": "value1",
					},
					"filter":    "false",
					"to_remove": "remove_me",
					"recombine": "true",
					"body":      "and the second one",
				},
			},
			Expected: map[string]interface{}{
				"body":      "This is first line\nand the second one",
				"filter":    "false",
				"recombine": "false",
				"field1":    "value1",
			},
			Processed: 2,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			processed := 0

			for _, body := range tt.Bodies {
				e := entry.New()
				e.Body = body

				p, err := pipeline.Operators()[0].Process(context.Background(), e)
				require.NoError(t, err)

				processed += p
			}

			assert.Equal(t, tt.Processed, processed)

			if tt.Processed == 0 {
				return
			}

			select {
			case e := <-fake.Received:
				assert.EqualValues(t, tt.Expected, e.Body)
				assert.EqualValues(t, map[string]interface{}{
					"field_copy_moved": "test",
					"new_field":        "test",
				}, e.Resource)
			case <-time.After(5 * time.Second):
				t.Logf("The entry should be flushed by now!")
				t.FailNow()
			}
		})
	}
}
