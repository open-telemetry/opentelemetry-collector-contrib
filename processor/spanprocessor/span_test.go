// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestNewTracesProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"foo"}
	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, component.ErrNilNextConsumer, err)
	require.Nil(t, tp)

	tp, err = factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)
}

// Common structure for the test cases.
type testCase struct {
	serviceName      string
	inputName        string
	inputAttributes  map[string]any
	outputName       string
	outputAttributes map[string]any
}

// runIndividualTestCase is the common logic of passing trace data through a configured attributes processor.
func runIndividualTestCase(t *testing.T, tt testCase, tp processor.Traces) {
	t.Run(tt.inputName, func(t *testing.T) {
		td := generateTraceData(tt.serviceName, tt.inputName, tt.inputAttributes)
		assert.NoError(t, tp.ConsumeTraces(context.Background(), td))
		assert.NoError(t, ptracetest.CompareTraces(generateTraceData(tt.serviceName, tt.outputName,
			tt.outputAttributes), td))
	})
}

func generateTraceData(serviceName, inputName string, attrs map[string]any) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	if serviceName != "" {
		rs.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName)
	}
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName(inputName)
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
	// TODO: Add test for "nil" Span. This needs support from data slices to allow to construct that.
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
	oCfg.Include = &filterconfig.MatchProperties{
		Config:   *createMatchConfig(filterset.Strict),
		Services: []string{"service"},
	}
	oCfg.Rename.FromAttributes = []string{"key"}

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)
	for i := range testCases {
		tt := testCases[i]
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tp.ConsumeTraces(context.Background(), tt.input))
			assert.NoError(t, ptracetest.CompareTraces(tt.output, tt.input))
		})
	}
}

// TestSpanProcessor_Values tests all possible value types.
func TestSpanProcessor_Values(t *testing.T) {
	// TODO: Add test for "nil" Span. This needs support from data slices to allow to construct that.
	testCases := []testCase{
		{
			inputName:        "",
			inputAttributes:  nil,
			outputName:       "",
			outputAttributes: nil,
		},
		{
			inputName:        "nil-attributes",
			inputAttributes:  nil,
			outputName:       "nil-attributes",
			outputAttributes: nil,
		},
		{
			inputName:        "empty-attributes",
			inputAttributes:  map[string]any{},
			outputName:       "empty-attributes",
			outputAttributes: map[string]any{},
		},
		{
			inputName: "string-type",
			inputAttributes: map[string]any{
				"key1": "bob",
			},
			outputName: "bob",
			outputAttributes: map[string]any{
				"key1": "bob",
			},
		},
		{
			inputName: "int-type",
			inputAttributes: map[string]any{
				"key1": 123,
			},
			outputName: "123",
			outputAttributes: map[string]any{
				"key1": 123,
			},
		},
		{
			inputName: "double-type",
			inputAttributes: map[string]any{
				"key1": 234.129312,
			},
			outputName: "234.129312",
			outputAttributes: map[string]any{
				"key1": 234.129312,
			},
		},
		{
			inputName: "bool-type",
			inputAttributes: map[string]any{
				"key1": true,
			},
			outputName: "true",
			outputAttributes: map[string]any{
				"key1": true,
			},
		},
		// TODO: What do we do when AttributeMap contains a nil entry? Is that possible?
		// TODO: In the new protocol do we want to support unknown type as 0 instead of string?
		// TODO: Do we want to allow constructing entries with unknown type?
		/*{
			inputName: "nil-type",
			inputAttributes: map[string]data.AttributeValue{
				"key1": data.NewAttributeValue(),
			},
			outputName: "<nil-attribute-value>",
			outputAttributes: map[string]data.AttributeValue{
				"key1": data.NewAttributeValue(),
			},
		},
		{
			inputName: "unknown-type",
			inputAttributes: map[string]data.AttributeValue{
				"key1": {},
			},
			outputName: "<unknown-attribute-type>",
			outputAttributes: map[string]data.AttributeValue{
				"key1": {},
			},
		},*/
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"key1"}

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)
	for _, tc := range testCases {
		runIndividualTestCase(t, tc, tp)
	}
}

// TestSpanProcessor_MissingKeys tests that missing a key in an attribute map results in no span name changes.
func TestSpanProcessor_MissingKeys(t *testing.T) {
	testCases := []testCase{
		{
			inputName: "first-keys-missing",
			inputAttributes: map[string]any{
				"key2": 123,
				"key3": 234.129312,
				"key4": true,
			},
			outputName: "first-keys-missing",
			outputAttributes: map[string]any{
				"key2": 123,
				"key3": 234.129312,
				"key4": true,
			},
		},
		{
			inputName: "middle-key-missing",
			inputAttributes: map[string]any{
				"key1": "bob",
				"key2": 123,
				"key4": true,
			},
			outputName: "middle-key-missing",
			outputAttributes: map[string]any{
				"key1": "bob",
				"key2": 123,
				"key4": true,
			},
		},
		{
			inputName: "last-key-missing",
			inputAttributes: map[string]any{
				"key1": "bob",
				"key2": 123,
				"key3": 234.129312,
			},
			outputName: "last-key-missing",
			outputAttributes: map[string]any{
				"key1": "bob",
				"key2": 123,
				"key3": 234.129312,
			},
		},
		{
			inputName: "all-keys-exists",
			inputAttributes: map[string]any{
				"key1": "bob",
				"key2": 123,
				"key3": 234.129312,
				"key4": true,
			},
			outputName: "bob::123::234.129312::true",
			outputAttributes: map[string]any{
				"key1": "bob",
				"key2": 123,
				"key3": 234.129312,
				"key4": true,
			},
		},
	}
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"key1", "key2", "key3", "key4"}
	oCfg.Rename.Separator = "::"

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)
	for _, tc := range testCases {
		runIndividualTestCase(t, tc, tp)
	}
}

// TestSpanProcessor_Separator ensures naming a span with a single key and separator will only contain the value from
// the single key.
func TestSpanProcessor_Separator(t *testing.T) {

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"key1"}
	oCfg.Rename.Separator = "::"

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	traceData := generateTraceData(
		"",
		"ensure no separator in the rename with one key",
		map[string]any{
			"key1": "bob",
		})
	assert.NoError(t, tp.ConsumeTraces(context.Background(), traceData))

	assert.NoError(t, ptracetest.CompareTraces(generateTraceData(
		"",
		"bob",
		map[string]any{
			"key1": "bob",
		}), traceData))
}

// TestSpanProcessor_NoSeparatorMultipleKeys tests naming a span using multiple keys and no separator.
func TestSpanProcessor_NoSeparatorMultipleKeys(t *testing.T) {

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"key1", "key2"}
	oCfg.Rename.Separator = ""

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	traceData := generateTraceData(
		"",
		"ensure no separator in the rename with two keys", map[string]any{
			"key1": "bob",
			"key2": 123,
		})
	assert.NoError(t, tp.ConsumeTraces(context.Background(), traceData))

	assert.NoError(t, ptracetest.CompareTraces(generateTraceData(
		"",
		"bob123",
		map[string]any{
			"key1": "bob",
			"key2": 123,
		}), traceData))
}

// TestSpanProcessor_SeparatorMultipleKeys tests naming a span with multiple keys and a separator.
func TestSpanProcessor_SeparatorMultipleKeys(t *testing.T) {

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"key1", "key2", "key3", "key4"}
	oCfg.Rename.Separator = "::"

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	traceData := generateTraceData(
		"",
		"rename with separators and multiple keys",
		map[string]any{
			"key1": "bob",
			"key2": 123,
			"key3": 234.129312,
			"key4": true,
		})
	assert.NoError(t, tp.ConsumeTraces(context.Background(), traceData))

	assert.NoError(t, ptracetest.CompareTraces(generateTraceData(
		"",
		"bob::123::234.129312::true",
		map[string]any{
			"key1": "bob",
			"key2": 123,
			"key3": 234.129312,
			"key4": true,
		}), traceData))
}

// TestSpanProcessor_NilName tests naming a span when the input span had no name.
func TestSpanProcessor_NilName(t *testing.T) {

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"key1"}
	oCfg.Rename.Separator = "::"

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	traceData := generateTraceData(
		"",
		"",
		map[string]any{
			"key1": "bob",
		})
	assert.NoError(t, tp.ConsumeTraces(context.Background(), traceData))

	assert.NoError(t, ptracetest.CompareTraces(generateTraceData(
		"",
		"bob",
		map[string]any{
			"key1": "bob",
		}), traceData))
}

// TestSpanProcessor_ToAttributes
func TestSpanProcessor_ToAttributes(t *testing.T) {

	testCases := []struct {
		rules           []string
		breakAfterMatch bool
		testCase
	}{
		{
			rules: []string{`^\/api\/v1\/document\/(?P<documentId>.*)\/update\/1$`},
			testCase: testCase{
				inputName:       "/api/v1/document/321083210/update/1",
				inputAttributes: map[string]any{},
				outputName:      "/api/v1/document/{documentId}/update/1",
				outputAttributes: map[string]any{
					"documentId": "321083210",
				},
			},
		},

		{
			rules: []string{`^\/api\/(?P<version>.*)\/document\/(?P<documentId>.*)\/update\/2$`},
			testCase: testCase{
				inputName:  "/api/v1/document/321083210/update/2",
				outputName: "/api/{version}/document/{documentId}/update/2",
				outputAttributes: map[string]any{
					"documentId": "321083210",
					"version":    "v1",
				},
			},
		},

		{
			rules: []string{`^\/api\/.*\/document\/(?P<documentId>.*)\/update\/3$`,
				`^\/api\/(?P<version>.*)\/document\/.*\/update\/3$`},
			testCase: testCase{
				inputName:  "/api/v1/document/321083210/update/3",
				outputName: "/api/{version}/document/{documentId}/update/3",
				outputAttributes: map[string]any{
					"documentId": "321083210",
					"version":    "v1",
				},
			},
			breakAfterMatch: false,
		},

		{
			rules: []string{`^\/api\/v1\/document\/(?P<documentId>.*)\/update\/4$`,
				`^\/api\/(?P<version>.*)\/document\/(?P<documentId>.*)\/update\/4$`},
			testCase: testCase{
				inputName:  "/api/v1/document/321083210/update/4",
				outputName: "/api/v1/document/{documentId}/update/4",
				outputAttributes: map[string]any{
					"documentId": "321083210",
				},
			},
			breakAfterMatch: true,
		},

		{
			rules: []string{"rule"},
			testCase: testCase{
				inputName:        "",
				outputName:       "",
				outputAttributes: nil,
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.ToAttributes = &ToAttributes{}

	for _, tc := range testCases {
		oCfg.Rename.ToAttributes.Rules = tc.rules
		oCfg.Rename.ToAttributes.BreakAfterMatch = tc.breakAfterMatch
		tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
		require.Nil(t, err)
		require.NotNil(t, tp)

		runIndividualTestCase(t, tc.testCase, tp)
	}
}

func TestSpanProcessor_skipSpan(t *testing.T) {
	testCases := []testCase{
		{
			serviceName: "bankss",
			inputName:   "url/url",
			outputName:  "url/url",
		},
		{
			serviceName: "banks",
			inputName:   "noslasheshere",
			outputName:  "noslasheshere",
		},
		{
			serviceName: "banks",
			inputName:   "www.test.com/code",
			outputName:  "{operation_website}",
			outputAttributes: map[string]any{
				"operation_website": "www.test.com/code",
			},
		},
		{
			serviceName: "banks",
			inputName:   "donot/",
			inputAttributes: map[string]any{
				"operation_website": "www.test.com/code",
			},
			outputName: "{operation_website}",
			outputAttributes: map[string]any{
				"operation_website": "donot/",
			},
		},
		{
			serviceName: "banks",
			inputName:   "donot/change",
			inputAttributes: map[string]any{
				"operation_website": "www.test.com/code",
			},
			outputName: "donot/change",
			outputAttributes: map[string]any{
				"operation_website": "www.test.com/code",
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Include = &filterconfig.MatchProperties{
		Config:    *createMatchConfig(filterset.Regexp),
		Services:  []string{`^banks$`},
		SpanNames: []string{"/"},
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		Config:    *createMatchConfig(filterset.Strict),
		SpanNames: []string{`donot/change`},
	}
	oCfg.Rename.ToAttributes = &ToAttributes{
		Rules: []string{`(?P<operation_website>.*?)$`},
	}
	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	for _, tc := range testCases {
		runIndividualTestCase(t, tc, tp)
	}
}

func generateTraceDataSetStatus(code ptrace.StatusCode, description string, attrs map[string]any) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Status().SetCode(code)
	span.Status().SetMessage(description)
	//nolint:errcheck
	span.Attributes().FromRaw(attrs)
	return td
}

func TestSpanProcessor_setStatusCode(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.SetStatus = &Status{
		Code:        "Error",
		Description: "Set custom error message",
	}
	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	td := generateTraceDataSetStatus(ptrace.StatusCodeUnset, "foobar", nil)

	assert.NoError(t, tp.ConsumeTraces(context.Background(), td))

	assert.EqualValues(t, generateTraceDataSetStatus(ptrace.StatusCodeError, "Set custom error message", nil), td)
}

func TestSpanProcessor_setStatusCodeConditionally(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.SetStatus = &Status{
		Code:        "Error",
		Description: "custom error message",
	}
	// This test number two include rule for applying rule only for status code 400
	oCfg.Include = &filterconfig.MatchProperties{
		Config: filterset.Config{
			MatchType: filterset.Strict,
		},
		Attributes: []filterconfig.Attribute{
			{Key: "http.status_code", Value: 400},
		},
	}
	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	testCases := []struct {
		inputAttributes         map[string]any
		inputStatusCode         ptrace.StatusCode
		outputStatusCode        ptrace.StatusCode
		outputStatusDescription string
	}{
		{
			// without attributes - should not apply rule and leave status code as it is
			inputStatusCode:  ptrace.StatusCodeOk,
			outputStatusCode: ptrace.StatusCodeOk,
		},
		{
			inputAttributes: map[string]any{
				"http.status_code": 400,
			},
			inputStatusCode:         ptrace.StatusCodeOk,
			outputStatusCode:        ptrace.StatusCodeError,
			outputStatusDescription: "custom error message",
		},
	}

	for _, tc := range testCases {
		t.Run("set-status-test", func(t *testing.T) {
			td := generateTraceDataSetStatus(tc.inputStatusCode, "", tc.inputAttributes)

			assert.NoError(t, tp.ConsumeTraces(context.Background(), td))

			assert.NoError(t, ptracetest.CompareTraces(generateTraceDataSetStatus(tc.outputStatusCode,
				tc.outputStatusDescription, tc.inputAttributes), td))
		})
	}
}
