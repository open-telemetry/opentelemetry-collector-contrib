// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"context"
	"io"
	"math/big"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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
				Body: map[string]interface{}{
					"message": "test_message",
				},
			},
			`body.message == "test_message"`,
			true,
		},
		{
			"NoMatchBody",
			&entry.Entry{
				Body: map[string]interface{}{
					"message": "invalid",
				},
			},
			`body.message == "test_message"`,
			false,
		},
		{
			"MatchAttribute",
			&entry.Entry{
				Body: map[string]interface{}{
					"message": "test_message",
				},
				Attributes: map[string]interface{}{
					"key": "value",
				},
			},
			`attributes.key == "value"`,
			true,
		},
		{
			"MatchBodyNested",
			&entry.Entry{
				Body: map[string]interface{}{
					"one": map[string]interface{}{
						"two": map[string]interface{}{
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
				Body: map[string]interface{}{
					"message": "test_message",
				},
				Attributes: map[string]interface{}{
					"one": map[string]interface{}{
						"two": map[string]interface{}{
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
				Body: map[string]interface{}{
					"message": "test_message",
				},
				Resource: map[string]interface{}{
					"one": map[string]interface{}{
						"two": map[string]interface{}{
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
				Body: map[string]interface{}{
					"message": "test_message",
				},
				Resource: map[string]interface{}{
					"one": map[string]interface{}{
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
				Body: map[string]interface{}{
					"message": "test_message",
				},
			},
			`attributes.key == "value"`,
			false,
		},
		{
			"MatchEnv",
			&entry.Entry{
				Body: map[string]interface{}{
					"message": "test_message",
				},
			},
			`env("TEST_FILTER_OPERATOR_ENV") == "foo"`,
			true,
		},
		{
			"NoMatchEnv",
			&entry.Entry{
				Body: map[string]interface{}{
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

			op, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			filtered := true
			mockOutput := testutil.NewMockOperator("output")
			mockOutput.On("Process", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				filtered = false
			})

			filterOperator, ok := op.(*Transformer)
			require.True(t, ok)

			filterOperator.OutputOperators = []operator.Operator{mockOutput}
			err = filterOperator.Process(context.Background(), tc.input)
			require.NoError(t, err)

			require.Equal(t, tc.filtered, filtered)
		})
	}
}

func TestFilterDropRatio(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.Expression = `body.message == "test_message"`
	cfg.DropRatio = 0.5
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	processedEntries := 0
	mockOutput := testutil.NewMockOperator("output")
	mockOutput.On("Process", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		processedEntries++
	})

	filterOperator, ok := op.(*Transformer)
	filterOperator.OutputOperators = []operator.Operator{mockOutput}
	require.True(t, ok)

	testEntry := &entry.Entry{
		Body: map[string]interface{}{
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
		err = filterOperator.Process(context.Background(), testEntry)
		require.NoError(t, err)
	}

	for i := 1; i < 11; i++ {
		err = filterOperator.Process(context.Background(), testEntry)
		require.NoError(t, err)
	}

	require.Equal(t, 10, processedEntries)
}
