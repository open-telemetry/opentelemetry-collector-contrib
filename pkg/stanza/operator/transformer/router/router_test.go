// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package router

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
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

			op, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			results := map[string]int{}
			var attributes map[string]any

			mock1 := testutil.NewMockOperator("output1")
			mock1.On("Process", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				results["output1"]++
				if entry, ok := args[1].(*entry.Entry); ok {
					attributes = entry.Attributes
				}
			})

			mock2 := testutil.NewMockOperator("output2")
			mock2.On("Process", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				results["output2"]++
				if entry, ok := args[1].(*entry.Entry); ok {
					attributes = entry.Attributes
				}
			})

			routerOperator := op.(*Transformer)
			err = routerOperator.SetOutputs([]operator.Operator{mock1, mock2})
			require.NoError(t, err)

			err = routerOperator.Process(context.Background(), tc.input)
			require.NoError(t, err)

			require.Equal(t, tc.expectedCounts, results)
			require.Equal(t, tc.expectedAttributes, attributes)
		})
	}
}
