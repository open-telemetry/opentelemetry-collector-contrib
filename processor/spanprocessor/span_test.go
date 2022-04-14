// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spanprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestNewTracesProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"foo"}
	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, nil)
	require.Error(t, componenterror.ErrNilNextConsumer, err)
	require.Nil(t, tp)

	tp, err = factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)
}

// Common structure for the test cases.
type testCase struct {
	serviceName      string
	inputName        string
	inputAttributes  map[string]interface{}
	outputName       string
	outputAttributes map[string]interface{}
}

// runIndividualTestCase is the common logic of passing trace data through a configured attributes processor.
func runIndividualTestCase(t *testing.T, tt testCase, tp component.TracesProcessor) {
	t.Run(tt.inputName, func(t *testing.T) {
		td := generateTraceData(tt.serviceName, tt.inputName, tt.inputAttributes)

		assert.NoError(t, tp.ConsumeTraces(context.Background(), td))
		// Ensure that the modified `td` has the attributes sorted:
		rss := td.ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			rs := rss.At(i)
			rs.Resource().Attributes().Sort()
			ilss := rs.ScopeSpans()
			for j := 0; j < ilss.Len(); j++ {
				spans := ilss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					spans.At(k).Attributes().Sort()
				}
			}
		}
		assert.EqualValues(t, generateTraceData(tt.serviceName, tt.outputName, tt.outputAttributes), td)
	})
}

func generateTraceData(serviceName, inputName string, attrs map[string]interface{}) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	if serviceName != "" {
		rs.Resource().Attributes().UpsertString(conventions.AttributeServiceName, serviceName)
	}
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName(inputName)
	pcommon.NewMapFromRaw(attrs).CopyTo(span.Attributes())
	span.Attributes().Sort()
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

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
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
			inputAttributes:  map[string]interface{}{},
			outputName:       "empty-attributes",
			outputAttributes: map[string]interface{}{},
		},
		{
			inputName: "string-type",
			inputAttributes: map[string]interface{}{
				"key1": "bob",
			},
			outputName: "bob",
			outputAttributes: map[string]interface{}{
				"key1": "bob",
			},
		},
		{
			inputName: "int-type",
			inputAttributes: map[string]interface{}{
				"key1": 123,
			},
			outputName: "123",
			outputAttributes: map[string]interface{}{
				"key1": 123,
			},
		},
		{
			inputName: "double-type",
			inputAttributes: map[string]interface{}{
				"key1": 234.129312,
			},
			outputName: "234.129312",
			outputAttributes: map[string]interface{}{
				"key1": 234.129312,
			},
		},
		{
			inputName: "bool-type",
			inputAttributes: map[string]interface{}{
				"key1": true,
			},
			outputName: "true",
			outputAttributes: map[string]interface{}{
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

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
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
			inputAttributes: map[string]interface{}{
				"key2": 123,
				"key3": 234.129312,
				"key4": true,
			},
			outputName: "first-keys-missing",
			outputAttributes: map[string]interface{}{
				"key2": 123,
				"key3": 234.129312,
				"key4": true,
			},
		},
		{
			inputName: "middle-key-missing",
			inputAttributes: map[string]interface{}{
				"key1": "bob",
				"key2": 123,
				"key4": true,
			},
			outputName: "middle-key-missing",
			outputAttributes: map[string]interface{}{
				"key1": "bob",
				"key2": 123,
				"key4": true,
			},
		},
		{
			inputName: "last-key-missing",
			inputAttributes: map[string]interface{}{
				"key1": "bob",
				"key2": 123,
				"key3": 234.129312,
			},
			outputName: "last-key-missing",
			outputAttributes: map[string]interface{}{
				"key1": "bob",
				"key2": 123,
				"key3": 234.129312,
			},
		},
		{
			inputName: "all-keys-exists",
			inputAttributes: map[string]interface{}{
				"key1": "bob",
				"key2": 123,
				"key3": 234.129312,
				"key4": true,
			},
			outputName: "bob::123::234.129312::true",
			outputAttributes: map[string]interface{}{
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

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
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

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	traceData := generateTraceData(
		"",
		"ensure no separator in the rename with one key",
		map[string]interface{}{
			"key1": "bob",
		})
	assert.NoError(t, tp.ConsumeTraces(context.Background(), traceData))

	assert.Equal(t, generateTraceData(
		"",
		"bob",
		map[string]interface{}{
			"key1": "bob",
		}), traceData)
}

// TestSpanProcessor_NoSeparatorMultipleKeys tests naming a span using multiple keys and no separator.
func TestSpanProcessor_NoSeparatorMultipleKeys(t *testing.T) {

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"key1", "key2"}
	oCfg.Rename.Separator = ""

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	traceData := generateTraceData(
		"",
		"ensure no separator in the rename with two keys", map[string]interface{}{
			"key1": "bob",
			"key2": 123,
		})
	assert.NoError(t, tp.ConsumeTraces(context.Background(), traceData))

	assert.Equal(t, generateTraceData(
		"",
		"bob123",
		map[string]interface{}{
			"key1": "bob",
			"key2": 123,
		}), traceData)
}

// TestSpanProcessor_SeparatorMultipleKeys tests naming a span with multiple keys and a separator.
func TestSpanProcessor_SeparatorMultipleKeys(t *testing.T) {

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"key1", "key2", "key3", "key4"}
	oCfg.Rename.Separator = "::"

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	traceData := generateTraceData(
		"",
		"rename with separators and multiple keys",
		map[string]interface{}{
			"key1": "bob",
			"key2": 123,
			"key3": 234.129312,
			"key4": true,
		})
	assert.NoError(t, tp.ConsumeTraces(context.Background(), traceData))

	assert.Equal(t, generateTraceData(
		"",
		"bob::123::234.129312::true",
		map[string]interface{}{
			"key1": "bob",
			"key2": 123,
			"key3": 234.129312,
			"key4": true,
		}), traceData)
}

// TestSpanProcessor_NilName tests naming a span when the input span had no name.
func TestSpanProcessor_NilName(t *testing.T) {

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Rename.FromAttributes = []string{"key1"}
	oCfg.Rename.Separator = "::"

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	traceData := generateTraceData(
		"",
		"",
		map[string]interface{}{
			"key1": "bob",
		})
	assert.NoError(t, tp.ConsumeTraces(context.Background(), traceData))

	assert.Equal(t, generateTraceData(
		"",
		"bob",
		map[string]interface{}{
			"key1": "bob",
		}), traceData)
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
				inputAttributes: map[string]interface{}{},
				outputName:      "/api/v1/document/{documentId}/update/1",
				outputAttributes: map[string]interface{}{
					"documentId": "321083210",
				},
			},
		},

		{
			rules: []string{`^\/api\/(?P<version>.*)\/document\/(?P<documentId>.*)\/update\/2$`},
			testCase: testCase{
				inputName:  "/api/v1/document/321083210/update/2",
				outputName: "/api/{version}/document/{documentId}/update/2",
				outputAttributes: map[string]interface{}{
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
				outputAttributes: map[string]interface{}{
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
				outputAttributes: map[string]interface{}{
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
		tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
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
			outputAttributes: map[string]interface{}{
				"operation_website": "www.test.com/code",
			},
		},
		{
			serviceName: "banks",
			inputName:   "donot/",
			inputAttributes: map[string]interface{}{
				"operation_website": "www.test.com/code",
			},
			outputName: "{operation_website}",
			outputAttributes: map[string]interface{}{
				"operation_website": "donot/",
			},
		},
		{
			serviceName: "banks",
			inputName:   "donot/change",
			inputAttributes: map[string]interface{}{
				"operation_website": "www.test.com/code",
			},
			outputName: "donot/change",
			outputAttributes: map[string]interface{}{
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
	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	for _, tc := range testCases {
		runIndividualTestCase(t, tc, tp)
	}
}

func generateTraceDataSetStatus(code ptrace.StatusCode, description string, attrs map[string]interface{}) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Status().SetCode(code)
	span.Status().SetMessage(description)
	pcommon.NewMapFromRaw(attrs).Sort().CopyTo(span.Attributes())
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
	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
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
	// This test numer two include rule for applying rule only for status code 400
	oCfg.Include = &filterconfig.MatchProperties{
		Config: filterset.Config{
			MatchType: filterset.Strict,
		},
		Attributes: []filterconfig.Attribute{
			{Key: "http.status_code", Value: 400},
		},
	}
	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	testCases := []struct {
		inputAttributes         map[string]interface{}
		inputStatusCode         ptrace.StatusCode
		outputStatusCode        ptrace.StatusCode
		outputStatusDescription string
	}{
		{
			// without attribiutes - should not apply rule and leave status code as it is
			inputStatusCode:  ptrace.StatusCodeOk,
			outputStatusCode: ptrace.StatusCodeOk,
		},
		{
			inputAttributes: map[string]interface{}{
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

			assert.EqualValues(t, generateTraceDataSetStatus(tc.outputStatusCode, tc.outputStatusDescription, tc.inputAttributes), td)
		})
	}
}
