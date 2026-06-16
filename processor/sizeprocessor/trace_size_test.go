// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sizeprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

// Common structure for all the Tests
type testCase struct {
	name               string
	serviceName        string
	inputAttributes    map[string]any
	expectedAttributes map[string]any
}

// runIndividualTestCase is the common logic of passing trace data through a configured attributes processor.
func runIndividualTestCase(t *testing.T, tt testCase, tp processor.Traces) {
	t.Run(tt.name, func(t *testing.T) {
		td := generateTraceData(tt.serviceName, tt.name, tt.inputAttributes)
		assert.NoError(t, tp.ConsumeTraces(context.Background(), td))
		require.NoError(t, ptracetest.CompareTraces(generateTraceData(tt.serviceName, tt.name, tt.expectedAttributes), td))
	})
}

func generateTraceData(serviceName, spanName string, attrs map[string]any) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	if serviceName != "" {
		rs.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName)
	}
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName(spanName)
	//nolint:errcheck
	span.Attributes().FromRaw(attrs)
	return td
}

// TestSpanProcessor_Values tests all possible value types.
func TestSpanProcessor_NilEmptyData(t *testing.T) {
	type nilEmptyTestCase struct {
		name   string
		input  ptrace.Traces
		output ptrace.Traces
	}
	// TODO: Add test for "nil" Span/Attributes. This needs support from data slices to allow to construct that.
	testCases := []nilEmptyTestCase{
		{
			name:   "empty",
			input:  ptrace.NewTraces(),
			output: ptrace.NewTraces(),
		},
		{
			name:   "one-empty-resource-spans",
			input:  testdata.GenerateTracesOneEmptyResourceSpans(),
			output: testdata.GenerateTracesOneEmptyResourceSpans(),
		},
		{
			name:   "no-libraries",
			input:  testdata.GenerateTracesNoLibraries(),
			output: testdata.GenerateTracesNoLibraries(),
		},
		{
			name:   "one-empty-instrumentation-library",
			input:  testdata.GenerateTracesOneEmptyInstrumentationLibrary(),
			output: testdata.GenerateTracesOneEmptyInstrumentationLibrary(),
		},
	}
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)

	tp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), oCfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tp.ConsumeTraces(context.Background(), tt.input))
			assert.EqualValues(t, tt.output, tt.input)
		})
	}
}

func TestAttributes_FilterSpans(t *testing.T) {
	testCases := []testCase{
		{
			name:            "apply processor",
			serviceName:     "svcB",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, tp)
	}
}
