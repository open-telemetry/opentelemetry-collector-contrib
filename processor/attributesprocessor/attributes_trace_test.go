// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
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
	oCfg.Settings.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
		{Key: "attribute1", Action: attraction.DELETE},
	}

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
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
		{
			name:        "apply processor with different value for exclude property",
			serviceName: "svcB",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect name for include property",
			serviceName:        "noname",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name:        "attribute match for exclude property",
			serviceName: "svcB",
			inputAttributes: map[string]any{
				"NoModification": true,
			},
			expectedAttributes: map[string]any{
				"NoModification": true,
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		Services: []string{"svcA", "svcB.*"},
		Config:   *createConfig(filterset.Regexp),
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		Attributes: []filterconfig.Attribute{
			{Key: "NoModification", Value: true},
		},
		Config: *createConfig(filterset.Strict),
	}
	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, tp)
	}
}

func TestAttributes_FilterSpansByNameStrict(t *testing.T) {
	testCases := []testCase{
		{
			name:            "apply",
			serviceName:     "svcB",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name:        "apply",
			serviceName: "svcB",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect_span_name",
			serviceName:        "svcB",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name:               "dont_apply",
			serviceName:        "svcB",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name:        "incorrect_span_name_with_attr",
			serviceName: "svcB",
			inputAttributes: map[string]any{
				"NoModification": true,
			},
			expectedAttributes: map[string]any{
				"NoModification": true,
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		SpanNames: []string{"apply", "dont_apply"},
		Config:    *createConfig(filterset.Strict),
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		SpanNames: []string{"dont_apply"},
		Config:    *createConfig(filterset.Strict),
	}
	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, tp)
	}
}

func TestAttributes_FilterSpansByNameRegexp(t *testing.T) {
	testCases := []testCase{
		{
			name:            "apply_to_span_with_no_attrs",
			serviceName:     "svcB",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name:        "apply_to_span_with_attr",
			serviceName: "svcB",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect_span_name",
			serviceName:        "svcB",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name:               "apply_dont_apply",
			serviceName:        "svcB",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name:        "incorrect_span_name_with_attr",
			serviceName: "svcB",
			inputAttributes: map[string]any{
				"NoModification": true,
			},
			expectedAttributes: map[string]any{
				"NoModification": true,
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		SpanNames: []string{"^apply.*"},
		Config:    *createConfig(filterset.Regexp),
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		SpanNames: []string{".*dont_apply$"},
		Config:    *createConfig(filterset.Regexp),
	}
	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, tp)
	}
}

func TestAttributes_Hash(t *testing.T) {
	testCases := []testCase{
		{
			name: "String",
			inputAttributes: map[string]any{
				"user.email": "john.doe@example.com",
			},
			expectedAttributes: map[string]any{
				"user.email": "836f82db99121b3481011f16b49dfa5fbc714a0d1b1b9f784a1ebbbf5b39577f",
			},
		},
		{
			name: "Int",
			inputAttributes: map[string]any{
				"user.id": 10,
			},
			expectedAttributes: map[string]any{
				"user.id": "a111f275cc2e7588000001d300a31e76336d15b9d314cd1a1d8f3d3556975eed",
			},
		},
		{
			name: "Double",
			inputAttributes: map[string]any{
				"user.balance": 99.1,
			},
			expectedAttributes: map[string]any{
				"user.balance": "05fabd78b01be9692863cb0985f600c99da82979af18db5c55173c2a30adb924",
			},
		},
		{
			name: "Bool",
			inputAttributes: map[string]any{
				"user.authenticated": true,
			},
			expectedAttributes: map[string]any{
				"user.authenticated": "4bf5122f344554c53bde2ebb8cd2b7e3d1600ad631c385a5d7cce23c7785459a",
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "user.email", Action: attraction.HASH},
		{Key: "user.id", Action: attraction.HASH},
		{Key: "user.balance", Action: attraction.HASH},
		{Key: "user.authenticated", Action: attraction.HASH},
	}

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, tp)
	}
}

func TestAttributes_Convert(t *testing.T) {
	testCases := []testCase{
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
			name: "true to int",
			inputAttributes: map[string]any{
				"to.int": true,
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
			name: "String to double (int-ish)",
			inputAttributes: map[string]any{
				"to.double": "123",
			},
			expectedAttributes: map[string]any{
				"to.double": 123.0,
			},
		},
		{
			name: "String to double (double-ish)",
			inputAttributes: map[string]any{
				"to.double": "123.6",
			},
			expectedAttributes: map[string]any{
				"to.double": 123.6,
			},
		},
		{
			name: "String to double (bad)",
			inputAttributes: map[string]any{
				"to.double": "int-10",
			},
			expectedAttributes: map[string]any{
				"to.double": "int-10",
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
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "to.int", Action: attraction.CONVERT, ConvertedType: "int"},
		{Key: "to.double", Action: attraction.CONVERT, ConvertedType: "double"},
		{Key: "to.string", Action: attraction.CONVERT, ConvertedType: "string"},
	}

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualTestCase(t, tt, tp)
	}
}

func BenchmarkAttributes_FilterSpansByName(b *testing.B) {
	testCases := []testCase{
		{
			name:            "apply_to_span_with_no_attrs",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name: "apply_to_span_with_attr",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "dont_apply",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		SpanNames: []string{"^apply.*"},
	}
	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(b, err)
	require.NotNil(b, tp)

	for _, tt := range testCases {
		td := generateTraceData(tt.serviceName, tt.name, tt.inputAttributes)

		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				assert.NoError(b, tp.ConsumeTraces(context.Background(), td))
			}
		})

		require.NoError(b, ptracetest.CompareTraces(generateTraceData(tt.serviceName, tt.name, tt.expectedAttributes), td))
	}
}

func createConfig(matchType filterset.MatchType) *filterset.Config {
	return &filterset.Config{
		MatchType: matchType,
	}
}
