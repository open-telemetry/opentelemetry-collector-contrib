// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sizeprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

// Common structure for all the Tests
type logTestCase struct {
	name               string
	inputAttributes    map[string]any
	expectedAttributes map[string]any
}

// runIndividualLogTestCase is the common logic of passing trace data through a configured attributes processor.
func runIndividualLogTestCase(t *testing.T, tt logTestCase, tp processor.Logs) {
	t.Run(tt.name, func(t *testing.T) {
		ld := generateLogData(tt.name, tt.inputAttributes)
		assert.NoError(t, tp.ConsumeLogs(context.Background(), ld))
		assert.NoError(t, plogtest.CompareLogs(generateLogData(tt.name, tt.expectedAttributes), ld))
	})
}

func generateLogData(resourceName string, attrs map[string]any) plog.Logs {
	td := plog.NewLogs()
	res := td.ResourceLogs().AppendEmpty()
	res.Resource().Attributes().PutStr("name", resourceName)
	sl := res.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	//nolint:errcheck
	lr.Attributes().FromRaw(attrs)
	return td
}

// TestLogProcessor_Values tests all possible value types.
func TestLogProcessor_NilEmptyData(t *testing.T) {
	type nilEmptyTestCase struct {
		name   string
		input  plog.Logs
		output plog.Logs
	}
	testCases := []nilEmptyTestCase{
		{
			name:   "empty",
			input:  plog.NewLogs(),
			output: plog.NewLogs(),
		},
		{
			name:   "one-empty-resource-logs",
			input:  testdata.GenerateLogsOneEmptyResourceLogs(),
			output: testdata.GenerateLogsOneEmptyResourceLogs(),
		},
		{
			name:   "no-libraries",
			input:  testdata.GenerateLogsOneEmptyResourceLogs(),
			output: testdata.GenerateLogsOneEmptyResourceLogs(),
		},
	}
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)

	tp, err := factory.CreateLogs(
		context.Background(),
		processortest.NewNopSettings(),
		oCfg,
		consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tp.ConsumeLogs(context.Background(), tt.input))
			assert.EqualValues(t, tt.output, tt.input)
		})
	}
}

func TestLogAttributes_Convert(t *testing.T) {
	testCases := []logTestCase{
		{
			name: "int to int",
			inputAttributes: map[string]any{
				"to.int": 1,
			},
			expectedAttributes: map[string]any{
				"to.int": 1,
			},
		},
		{
			name: "false to int",
			inputAttributes: map[string]any{
				"to.int": false,
			},
			expectedAttributes: map[string]any{
				"to.int": 0,
			},
		},
		{
			name: "String to int (good)",
			inputAttributes: map[string]any{
				"to.int": "123",
			},
			expectedAttributes: map[string]any{
				"to.int": 123,
			},
		},
		{
			name: "String to int (bad)",
			inputAttributes: map[string]any{
				"to.int": "int-10",
			},
			expectedAttributes: map[string]any{
				"to.int": "int-10",
			},
		},
		{
			name: "String to double",
			inputAttributes: map[string]any{
				"to.double": "123.6",
			},
			expectedAttributes: map[string]any{
				"to.double": 123.6,
			},
		},
		{
			name: "Double to string",
			inputAttributes: map[string]any{
				"to.string": 99.1,
			},
			expectedAttributes: map[string]any{
				"to.string": "99.1",
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualLogTestCase(t, tt, tp)
	}
}
