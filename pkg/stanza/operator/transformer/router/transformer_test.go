// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package router

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/stanzaerrors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestTransformer(t *testing.T) {
	t.Setenv("TEST_ROUTER_OPERATOR_ENV", "foo")

	basicConfig := func() *Config {
		return &Config{
			BasicConfig: helper.BasicConfig{
				OperatorID:   "test_operator_id",
				OperatorType: "router",
			},
		}
	}

	cases := []struct {
		name               string
		input              *entry.Entry
		routes             []*RouteConfig
		defaultOutput      []string
		expectedCounts     map[string]int
		expectedAttributes map[string]any
	}{
		{
			"DefaultRoute",
			entry.New(),
			[]*RouteConfig{
				{
					helper.NewAttributerConfig(),
					"true",
					[]string{"output1"},
				},
			},
			nil,
			map[string]int{"output1": 1},
			nil,
		},
		{
			"NoMatch",
			entry.New(),
			[]*RouteConfig{
				{
					helper.NewAttributerConfig(),
					`false`,
					[]string{"output1"},
				},
			},
			nil,
			map[string]int{},
			nil,
		},
		{
			"SimpleMatch",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
			},
			[]*RouteConfig{
				{
					helper.NewAttributerConfig(),
					`body.message == "non_match"`,
					[]string{"output1"},
				},
				{
					helper.NewAttributerConfig(),
					`body.message == "test_message"`,
					[]string{"output2"},
				},
			},
			nil,
			map[string]int{"output2": 1},
			nil,
		},
		{
			"MatchWithAttribute",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
			},
			[]*RouteConfig{
				{
					helper.NewAttributerConfig(),
					`body.message == "non_match"`,
					[]string{"output1"},
				},
				{
					helper.AttributerConfig{
						Attributes: map[string]helper.ExprStringConfig{
							"label-key": "label-value",
						},
					},
					`body.message == "test_message"`,
					[]string{"output2"},
				},
			},
			nil,
			map[string]int{"output2": 1},
			map[string]any{
				"label-key": "label-value",
			},
		},
		{
			"MatchEnv",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
			},
			[]*RouteConfig{
				{
					helper.NewAttributerConfig(),
					`env("TEST_ROUTER_OPERATOR_ENV") == "foo"`,
					[]string{"output1"},
				},
				{
					helper.NewAttributerConfig(),
					`true`,
					[]string{"output2"},
				},
			},
			nil,
			map[string]int{"output1": 1},
			nil,
		},
		{
			"UseDefault",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
			},
			[]*RouteConfig{
				{
					helper.NewAttributerConfig(),
					`false`,
					[]string{"output1"},
				},
			},
			[]string{"output2"},
			map[string]int{"output2": 1},
			nil,
		},
		{
			"MatchBeforeDefault",
			&entry.Entry{
				Body: map[string]any{
					"message": "test_message",
				},
			},
			[]*RouteConfig{
				{
					helper.NewAttributerConfig(),
					`true`,
					[]string{"output1"},
				},
			},
			[]string{"output2"},
			map[string]int{"output1": 1},
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := basicConfig()
			cfg.Routes = tc.routes
			cfg.Default = tc.defaultOutput

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			results := map[string]int{}
			var attributes map[string]any

			mock1 := testutil.NewMockOperator("output1")
			mock1.On(
				"ProcessBatch", mock.Anything, mock.Anything,
			).Return(
				stanzaerrors.NewError("Operator can not process logs.", ""),
			).Run(func(args mock.Arguments) {
				entries := args.Get(1).([]*entry.Entry)
				results["output1"] += len(entries)
				if len(entries) > 0 {
					attributes = entries[0].Attributes
				}
			})

			mock2 := testutil.NewMockOperator("output2")
			mock2.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				entries := args.Get(1).([]*entry.Entry)
				results["output2"] += len(entries)
				if len(entries) > 0 {
					attributes = entries[0].Attributes
				}
			})

			err = op.SetOutputs([]operator.Operator{mock1, mock2})
			require.NoError(t, err)

			err = op.ProcessBatch(t.Context(), []*entry.Entry{tc.input})
			require.NoError(t, err)

			require.Equal(t, tc.expectedCounts, results)
			require.Equal(t, tc.expectedAttributes, attributes)
		})
	}
}

func TestRouterDoesNotSplitBatches(t *testing.T) {
	cfg := &Config{
		BasicConfig: helper.BasicConfig{
			OperatorID:   "test_router",
			OperatorType: "router",
		},
		Routes: []*RouteConfig{
			{
				helper.NewAttributerConfig(),
				`body.route == "route1"`,
				[]string{"output1"},
			},
			{
				helper.NewAttributerConfig(),
				`body.route == "route2"`,
				[]string{"output2"},
			},
		},
	}

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mock1 := testutil.NewMockOperator("output1")
	mock1.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

	mock2 := testutil.NewMockOperator("output2")
	mock2.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

	err = op.SetOutputs([]operator.Operator{mock1, mock2})
	require.NoError(t, err)

	testEntries := []*entry.Entry{
		{Body: map[string]any{"route": "route1"}},
		{Body: map[string]any{"route": "route1"}},
		{Body: map[string]any{"route": "route2"}},
		{Body: map[string]any{"route": "route1"}},
		{Body: map[string]any{"route": "route2"}},
		{Body: map[string]any{"route": "route2"}},
	}

	err = op.ProcessBatch(t.Context(), testEntries)
	require.NoError(t, err)

	// Verify that each output's ProcessBatch was called exactly once
	// This proves that batches were not split by the router
	mock1.AssertNumberOfCalls(t, "ProcessBatch", 1)
	mock2.AssertNumberOfCalls(t, "ProcessBatch", 1)

	// Verify the correct number of entries were routed to each output
	// Filter calls to only ProcessBatch calls
	calls1 := []mock.Call{}
	for _, call := range mock1.Calls {
		if call.Method == "ProcessBatch" {
			calls1 = append(calls1, call)
		}
	}
	require.Len(t, calls1, 1)
	passedEntries1 := calls1[0].Arguments.Get(1).([]*entry.Entry)
	require.Len(t, passedEntries1, 3) // Three entries match route1

	calls2 := []mock.Call{}
	for _, call := range mock2.Calls {
		if call.Method == "ProcessBatch" {
			calls2 = append(calls2, call)
		}
	}
	require.Len(t, calls2, 1)
	passedEntries2 := calls2[0].Arguments.Get(1).([]*entry.Entry)
	require.Len(t, passedEntries2, 3) // Three entries match route2
}

func TestProcessBatchContinuesOnExpressionError(t *testing.T) {
	// This test verifies that when vm.Run fails for one entry,
	// other entries in the batch are still processed correctly.
	cfg := &Config{
		BasicConfig: helper.BasicConfig{
			OperatorID:   "test_router",
			OperatorType: "router",
		},
		Routes: []*RouteConfig{
			{
				helper.NewAttributerConfig(),
				// This expression will fail when body.value is not a number
				`body.value > 10`,
				[]string{"output1"},
			},
			{
				helper.NewAttributerConfig(),
				`true`, // Fallback route
				[]string{"output2"},
			},
		},
	}

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mock1 := testutil.NewMockOperator("output1")
	mock1.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

	mock2 := testutil.NewMockOperator("output2")
	mock2.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

	err = op.SetOutputs([]operator.Operator{mock1, mock2})
	require.NoError(t, err)

	testEntries := []*entry.Entry{
		{Body: map[string]any{"value": 20}},             // Matches route1 (value > 10)
		{Body: map[string]any{"value": "not_a_number"}}, // Expression error, falls through to route2
		{Body: map[string]any{"value": 5}},              // value <= 10, matches route2
		{Body: map[string]any{"value": 15}},             // Matches route1 (value > 10)
	}

	err = op.ProcessBatch(t.Context(), testEntries)
	require.NoError(t, err)

	// Verify both outputs received entries
	mock1.AssertCalled(t, "ProcessBatch", mock.Anything, mock.Anything)
	mock2.AssertCalled(t, "ProcessBatch", mock.Anything, mock.Anything)

	// Get the entries passed to each output
	calls1 := []mock.Call{}
	for _, call := range mock1.Calls {
		if call.Method == "ProcessBatch" {
			calls1 = append(calls1, call)
		}
	}
	require.Len(t, calls1, 1)
	passedEntries1 := calls1[0].Arguments.Get(1).([]*entry.Entry)
	require.Len(t, passedEntries1, 2) // Two entries match route1 (value 20 and 15)

	calls2 := []mock.Call{}
	for _, call := range mock2.Calls {
		if call.Method == "ProcessBatch" {
			calls2 = append(calls2, call)
		}
	}
	require.Len(t, calls2, 1)
	passedEntries2 := calls2[0].Arguments.Get(1).([]*entry.Entry)
	require.Len(t, passedEntries2, 2) // Two entries match route2 (expression error and value 5)
}

func TestProcessBatchContinuesOnAttributeError(t *testing.T) {
	// This test verifies that when route.Attribute fails for one entry,
	// other entries in the batch are still processed correctly.
	cfg := &Config{
		BasicConfig: helper.BasicConfig{
			OperatorID:   "test_router",
			OperatorType: "router",
		},
		Routes: []*RouteConfig{
			{
				helper.AttributerConfig{
					Attributes: map[string]helper.ExprStringConfig{
						// This will fail when body.label is not a string
						"dynamic-label": `EXPR(body.label)`,
					},
				},
				`true`,
				[]string{"output1"},
			},
		},
	}

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mock1 := testutil.NewMockOperator("output1")
	mock1.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

	err = op.SetOutputs([]operator.Operator{mock1})
	require.NoError(t, err)

	testEntries := []*entry.Entry{
		{Body: map[string]any{"label": "valid_string"}},  // Should succeed
		{Body: map[string]any{"label": 12345}},           // Attribute error (non-string)
		{Body: map[string]any{"label": "another_valid"}}, // Should succeed
	}

	err = op.ProcessBatch(t.Context(), testEntries)
	require.NoError(t, err) // Batch should complete without returning error

	// Verify output1 received the valid entries (skipping the one with attribute error)
	mock1.AssertCalled(t, "ProcessBatch", mock.Anything, mock.Anything)

	calls1 := []mock.Call{}
	for _, call := range mock1.Calls {
		if call.Method == "ProcessBatch" {
			calls1 = append(calls1, call)
		}
	}
	require.Len(t, calls1, 1)
	passedEntries1 := calls1[0].Arguments.Get(1).([]*entry.Entry)
	require.Len(t, passedEntries1, 2) // Only the two valid entries should be routed
}
