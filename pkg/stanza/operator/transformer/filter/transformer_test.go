// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"io"
	"math/big"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestTransformer(t *testing.T) {
	t.Setenv("TEST_FILTER_OPERATOR_ENV", "foo")

	cases := []struct {
		name       string
		input      *entry.Entry
		expression string
		filtered   bool
	}{
		{
			"BodyMatch",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
			},
			`body.message == "test_message"`,
			true,
		},
		{
			"NoMatchBody",
			&entry.Entry{
				Body: map[string]any{
					"message": "invalid",
				},
			},
			`body.message == "test_message"`,
			false,
		},
		{
			"FilterOutRegexp",
			&entry.Entry{
				Body: map[string]any{
					"message": "INFO: this is an info message",
				},
			},
			`body.message matches "^INFO:"`,
			true,
		},
		{
			"FilterInRegexp",
			&entry.Entry{
				Body: map[string]any{
					"message": "WARN: this is a warning message",
				},
			},
			`body.message not matches "^WARN:"`,
			false,
		},
		{
			"MatchAttribute",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
				Attributes: map[string]any{
					"key": "value",
				},
			},
			`attributes.key == "value"`,
			true,
		},
		{
			"MatchBodyNested",
			&entry.Entry{
				Body: map[string]any{
					"one": map[string]any{
						"two": map[string]any{
							"key": "value",
						},
					},
				},
			},
			`body.one.two.key == "value"`,
			true,
		},
		{
			"MatchAttributeNested",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
				Attributes: map[string]any{
					"one": map[string]any{
						"two": map[string]any{
							"key": "value",
						},
					},
				},
			},
			`attributes.one.two.key == "value"`,
			true,
		},
		{
			"MatchResourceNested",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
				Resource: map[string]any{
					"one": map[string]any{
						"two": map[string]any{
							"key": "value",
						},
					},
				},
			},
			`resource.one.two.key == "value"`,
			true,
		},
		{
			"MatchResourceBracketed",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
				Resource: map[string]any{
					"one": map[string]any{
						"two.stilltwo": "value",
					},
				},
			},
			`resource.one["two.stilltwo"] == "value"`,
			true,
		},
		{
			"NoMatchAttribute",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
			},
			`attributes.key == "value"`,
			false,
		},
		{
			"MatchEnv",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
			},
			`env("TEST_FILTER_OPERATOR_ENV") == "foo"`,
			true,
		},
		{
			"NoMatchEnv",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
			},
			`env("TEST_FILTER_OPERATOR_ENV") == "bar"`,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfigWithID("test")
			cfg.Expression = tc.expression

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			filtered := true
			mockOutput := testutil.NewMockOperator("output")
			mockOutput.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				entries := args.Get(1).([]*entry.Entry)
				if len(entries) > 0 {
					filtered = false
				}
			})

			op.(*Transformer).OutputOperators = []operator.Operator{mockOutput}
			err = op.ProcessBatch(t.Context(), []*entry.Entry{tc.input})
			require.NoError(t, err)

			require.Equal(t, tc.filtered, filtered)
		})
	}
}

func TestFilterDropRatio(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.Expression = `body.message == "test_message"`
	cfg.DropRatio = 0.5
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	processedEntries := 0
	mockOutput := testutil.NewMockOperator("output")
	mockOutput.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		entries := args.Get(1).([]*entry.Entry)
		processedEntries += len(entries)
	})

	filterOperator, ok := op.(*Transformer)
	filterOperator.OutputOperators = []operator.Operator{mockOutput}
	require.True(t, ok)

	testEntry := &entry.Entry{
		Body: map[string]any{
			"message": "test_message",
		},
	}

	nextIndex := 0
	randos := []int64{250, 750}
	randInt = func(io.Reader, *big.Int) (*big.Int, error) {
		defer func() {
			nextIndex = (nextIndex + 1) % len(randos)
		}()
		return big.NewInt(randos[nextIndex]), nil
	}

	for i := 1; i < 11; i++ {
		err = filterOperator.ProcessBatch(t.Context(), []*entry.Entry{testEntry})
		require.NoError(t, err)
	}

	for i := 1; i < 11; i++ {
		err = filterOperator.ProcessBatch(t.Context(), []*entry.Entry{testEntry})
		require.NoError(t, err)
	}

	require.Equal(t, 10, processedEntries)
}

func TestFilterDoesNotSplitBatches(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.Expression = `body.message == "drop_me"`
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mockOutput := testutil.NewMockOperator("output")
	mockOutput.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

	filterOperator := op.(*Transformer)
	filterOperator.OutputOperators = []operator.Operator{mockOutput}

	testEntries := []*entry.Entry{
		{Body: map[string]any{"message": "keep_me"}},
		{Body: map[string]any{"message": "keep_me_too"}},
		{Body: map[string]any{"message": "drop_me"}},
		{Body: map[string]any{"message": "also_keep"}},
	}

	err = filterOperator.ProcessBatch(t.Context(), testEntries)
	require.NoError(t, err)

	// Verify that ProcessBatch was called exactly once, proving the batch was not split
	mockOutput.AssertNumberOfCalls(t, "ProcessBatch", 1)

	// Verify that 3 entries were passed (one was filtered out)
	calls := mockOutput.Calls
	require.Len(t, calls, 1)
	passedEntries := calls[0].Arguments.Get(1).([]*entry.Entry)
	require.Len(t, passedEntries, 3)
}
