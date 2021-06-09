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

package filter

import (
	"context"
	"io"
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestFilterOperator(t *testing.T) {
	os.Setenv("TEST_FILTER_PLUGIN_ENV", "foo")
	defer os.Unsetenv("TEST_FILTER_PLUGIN_ENV")

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
			`$.message == "test_message"`,
			true,
		},
		{
			"NoMatchBody",
			&entry.Entry{
				Body: map[string]interface{}{
					"message": "invalid",
				},
			},
			`$.message == "test_message"`,
			false,
		},
		{
			"MatchAttribute",
			&entry.Entry{
				Body: map[string]interface{}{
					"message": "test_message",
				},
				Attributes: map[string]string{
					"key": "value",
				},
			},
			`$attributes.key == "value"`,
			true,
		},
		{
			"NoMatchAttribute",
			&entry.Entry{
				Body: map[string]interface{}{
					"message": "test_message",
				},
			},
			`$attributes.key == "value"`,
			false,
		},
		{
			"MatchEnv",
			&entry.Entry{
				Body: map[string]interface{}{
					"message": "test_message",
				},
			},
			`env("TEST_FILTER_PLUGIN_ENV") == "foo"`,
			true,
		},
		{
			"NoMatchEnv",
			&entry.Entry{
				Body: map[string]interface{}{
					"message": "test_message",
				},
			},
			`env("TEST_FILTER_PLUGIN_ENV") == "bar"`,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewFilterOperatorConfig("test")
			cfg.Expression = tc.expression

			buildContext := testutil.NewBuildContext(t)
			ops, err := cfg.Build(buildContext)
			require.NoError(t, err)
			op := ops[0]

			filtered := true
			mockOutput := testutil.NewMockOperator("output")
			mockOutput.On("Process", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				filtered = false
			})

			filterOperator, ok := op.(*FilterOperator)
			require.True(t, ok)

			filterOperator.OutputOperators = []operator.Operator{mockOutput}
			err = filterOperator.Process(context.Background(), tc.input)
			require.NoError(t, err)

			require.Equal(t, tc.filtered, filtered)
		})
	}
}

func TestFilterDropRatio(t *testing.T) {
	cfg := NewFilterOperatorConfig("test")
	cfg.Expression = `$.message == "test_message"`
	cfg.DropRatio = 0.5
	buildContext := testutil.NewBuildContext(t)
	ops, err := cfg.Build(buildContext)
	require.NoError(t, err)
	op := ops[0]

	processedEntries := 0
	mockOutput := testutil.NewMockOperator("output")
	mockOutput.On("Process", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		processedEntries++
	})

	filterOperator, ok := op.(*FilterOperator)
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
