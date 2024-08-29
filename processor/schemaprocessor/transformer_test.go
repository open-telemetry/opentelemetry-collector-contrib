// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"
)
// todo(ankit) test for all schema changes

func newTestTransformer(t *testing.T) *transformer {
	trans, err := newTransformer(context.Background(), newDefaultConfiguration(), processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zaptest.NewLogger(t),
		},
	})
	require.NoError(t, err, "Must not error when creating default transformer")
	return trans
}

func TestTransformerStart(t *testing.T) {
	t.Parallel()

	trans := newTestTransformer(t)
	assert.NoError(t, trans.start(context.Background(), componenttest.NewNopHost()))
}

//go:embed testdata/testschemas
var testdataFiles embed.FS

type SchemaResource interface {
	SetSchemaUrl(schema string)
	SchemaUrl() string
}

type SchemaResourceIterable[T SchemaResource] interface {
	At(int) T
	Len() int
}

func setSchemaForAllItems[T SchemaResource](iterable SchemaResourceIterable[T], schemaURL string) {
	for i := 0; i < iterable.Len(); i++ {
		item := iterable.At(i)
		item.SetSchemaUrl(schemaURL)
	}
}

//go:embed testdata
var f embed.FS

func plogsFromJson(t *testing.T, path string) plog.Logs {
	t.Helper()
	unmarshaler := plog.JSONUnmarshaler{}
	logJSON, err := f.ReadFile(path)
	require.NoError(t, err)
	inLogs, err := unmarshaler.UnmarshalLogs(logJSON)
	require.NoError(t, err)
	return inLogs
}

func buildTestTransformer(t *testing.T, targetUrl string)  *transformer {
	t.Helper()
	defaultConfig := newDefaultConfiguration()
	castedConfig := defaultConfig.(*Config)
	castedConfig.Targets = []string{targetUrl}
	transform, err := newTransformer(context.Background(), castedConfig, processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zaptest.NewLogger(t),
		},
	})
	require.NoError(t, err, "Must not error when creating transformer")
	err = transform.manager.SetProviders(translation.NewTestProvider(&testdataFiles))
	require.NoError(t, err)
	return transform
}

func TestTransformerSchemaBySections(t *testing.T) {
	tests := []struct {
		name            string
		section           string
		dataType       component.DataType
	}{
		{
			// todo(ankit) do i need to test all data types here?
			name: "all_log",
			section: "all",
			dataType: component.DataTypeLogs,
		},
		{
			// todo(ankit) do i need to test all data types here?
			name: "resource_log",
			section: "resource",
			dataType: component.DataTypeLogs,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			transform := buildTestTransformer(t, fmt.Sprintf("https://example.com/testdata/testschemas/schema_sections/%s/1.0.0", tc.section))

			inLogs := plogsFromJson(t, fmt.Sprintf("testdata/transformer_data/schema_sections/%s/%s_in.json", tc.section, tc.dataType))

			expectedLogs := plogsFromJson(t, fmt.Sprintf("testdata/transformer_data/schema_sections/%s/%s_out.json", tc.section, tc.dataType))

			logs, err := transform.processLogs(context.Background(), inLogs)
			assert.NoError(t, err)
			assert.EqualValues(t, expectedLogs, logs, "Must match the expected values")
		})

	}
}

func TestTransformerProcessing(t *testing.T) {
	t.Parallel()

	trans := newTestTransformer(t)
	t.Run("metrics", func(t *testing.T) {
		in := pmetric.NewMetrics()
		in.ResourceMetrics().AppendEmpty()
		in.ResourceMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
		in.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
		m := in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
		m.SetName("test-data")
		m.SetDescription("Only used throughout tests")
		m.SetUnit("seconds")
		m.CopyTo(in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0))

		out, err := trans.processMetrics(context.Background(), in)
		assert.NoError(t, err, "Must not error when processing metrics")
		assert.Equal(t, in, out, "Must return the same data (subject to change)")
	})

	t.Run("traces", func(t *testing.T) {
		in := ptrace.NewTraces()
		in.ResourceSpans().AppendEmpty()
		in.ResourceSpans().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
		in.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
		s := in.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
		s.SetName("http.request")
		s.SetKind(ptrace.SpanKindConsumer)
		s.SetSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})
		s.CopyTo(in.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0))

		out, err := trans.processTraces(context.Background(), in)
		assert.NoError(t, err, "Must not error when processing metrics")
		assert.Equal(t, in, out, "Must return the same data (subject to change)")
	})

	t.Run("logs", func(t *testing.T) {
		in := plog.NewLogs()
		in.ResourceLogs().AppendEmpty()
		in.ResourceLogs().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
		in.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
		l := in.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
		l.SetName("magical-logs")
		l.SetVersion("alpha")
		l.CopyTo(in.ResourceLogs().At(0).ScopeLogs().At(0).Scope())

		out, err := trans.processLogs(context.Background(), in)
		assert.NoError(t, err, "Must not error when processing metrics")
		assert.Equal(t, in, out, "Must return the same data (subject to change)")
	})
}



type SchemaUsed int

const (
	ResourceSchemaVersionUsed = iota + 1
	ScopeSchemaVersionUsed
	NoopSchemaUsed
)

// returns a test log with no schema versions set
func GenerateLogForTest() plog.Logs {
	in := plog.NewLogs()
	in.ResourceLogs().AppendEmpty()
	in.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
	l := in.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
	l.Attributes().PutStr("input", "test")
	return in
}

 // case 1: resource schema set, scope schema not set, use resource schema
 // case 2: resource schema not set, scope schema set, use scope schema inside
 // case 3: resource schema set, scope schema set, use scope schema
 // case 4: resource schema not set, scope schema not set, noop translation
func TestTransformerScopeLogSchemaPrecedence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   func() plog.Logs
		whichSchemaUsed    SchemaUsed
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "resourcesetscopeunset",
			input: func() plog.Logs {
				log := GenerateLogForTest()
				log.ResourceLogs().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				return log
			},
			whichSchemaUsed: ResourceSchemaVersionUsed,
			wantErr: assert.NoError,
		},
		{
		    name: "resourceunsetscopeset",
		    input: func() plog.Logs {
		            log := GenerateLogForTest()
		            log.ResourceLogs().At(0).ScopeLogs().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")
		            return log
		    },
			whichSchemaUsed: ScopeSchemaVersionUsed,
		    wantErr: assert.NoError,
		},
		{
		    name: "resourcesetscopeset",
		    input: func() plog.Logs {
		            log := GenerateLogForTest()
		            log.ResourceLogs().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
					 log.ResourceLogs().At(0).ScopeLogs().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")

				 return log
		    },
			whichSchemaUsed: ScopeSchemaVersionUsed,
		    wantErr: assert.NoError,
		},
		{
		     name: "resourceunsetscopeunset",
		     input: func() plog.Logs {
		             log := GenerateLogForTest()
		             return log
		     },
		     //want: "https://example.com/testdata/testschemas/schemaprecedence/1.0.0",
			whichSchemaUsed: NoopSchemaUsed,
			wantErr: assert.NoError,
		},
		}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultConfig := newDefaultConfiguration()
			castedConfig := defaultConfig.(*Config)
			castedConfig.Targets = []string{"https://example.com/testdata/testschemas/schemaprecedence/1.2.0"}
			transform, err := newTransformer(context.Background(), defaultConfig, processor.Settings{
				TelemetrySettings: component.TelemetrySettings{
					Logger: zaptest.NewLogger(t),
				},
			})
			require.NoError(t, err, "Must not error when creating transformer")

			err = transform.manager.SetProviders(translation.NewTestProvider(&testdataFiles))
			require.NoError(t, err)
			got, err := transform.processLogs(context.Background(), tt.input())
			if !tt.wantErr(t, err, fmt.Sprintf("processLogs(%v)", tt.input())) {
				return
			}
			targetLog := got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			assert.Equal(t, targetLog.Attributes().Len(), 1)
			_, usedResource := targetLog.Attributes().Get("one_one_zero_output")
			_, usedScope := targetLog.Attributes().Get("one_two_zero_output")
			_, usedNoop := targetLog.Attributes().Get("input")
			switch tt.whichSchemaUsed {
			case ResourceSchemaVersionUsed:
				assert.True(t, usedResource, "processLogs(%v) not using correct schema,, attributes present: %v", tt.name, targetLog.Attributes().AsRaw())
			case ScopeSchemaVersionUsed:
				assert.True(t, usedScope, "processLogs(%v) not using correct schema, attributes present: %v", tt.name, targetLog.Attributes().AsRaw())
			case NoopSchemaUsed:
				assert.True(t, usedNoop, "processLogs(%v) not using correct schema,, attributes present: %v", tt.name, targetLog.Attributes().AsRaw())
			}
		})
	}
}

// returns a test trace with no schema versions set
func GenerateTraceForTest() ptrace.Traces {
	in := ptrace.NewTraces()
	in.ResourceSpans().AppendEmpty()
	in.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
	l := in.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
	l.Attributes().PutStr("input", "test")
	return in
}

// literally the exact same tests as above
// case 1: resource schema set, scope schema not set, use resource schema
// case 2: resource schema not set, scope schema set, use scope schema inside
// case 3: resource schema set, scope schema set, use scope schema
// case 4: resource schema not set, scope schema not set, noop translation
func TestTransformerScopeTraceSchemaPrecedence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   func() ptrace.Traces
		whichSchemaUsed    SchemaUsed
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "resourcesetscopeunset",
			input: func() ptrace.Traces {
				trace := GenerateTraceForTest()
				trace.ResourceSpans().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				return trace
			},
			whichSchemaUsed: ResourceSchemaVersionUsed,
			wantErr: assert.NoError,
		},
		{
			name: "resourceunsetscopeset",
			input: func() ptrace.Traces {
				trace := GenerateTraceForTest()
				trace.ResourceSpans().At(0).ScopeSpans().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")
				return trace
			},
			whichSchemaUsed: ScopeSchemaVersionUsed,
			wantErr: assert.NoError,
		},
		{
			name: "resourcesetscopeset",
			input: func() ptrace.Traces {
				trace := GenerateTraceForTest()
				trace.ResourceSpans().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				trace.ResourceSpans().At(0).ScopeSpans().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")

				return trace
			},
			whichSchemaUsed: ScopeSchemaVersionUsed,
			wantErr: assert.NoError,
		},
		{
			name: "resourceunsetscopeunset",
			input: func() ptrace.Traces {
				trace := GenerateTraceForTest()
				return trace
			},
			//want: "https://example.com/testdata/testschemas/schemaprecedence/1.0.0",
			whichSchemaUsed: NoopSchemaUsed,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultConfig := newDefaultConfiguration()
			castedConfig := defaultConfig.(*Config)
			castedConfig.Targets = []string{"https://example.com/testdata/testschemas/schemaprecedence/1.2.0"}
			transform, err := newTransformer(context.Background(), defaultConfig, processor.Settings{
				TelemetrySettings: component.TelemetrySettings{
					Logger: zaptest.NewLogger(t),
				},
			})
			require.NoError(t, err, "Must not error when creating transformer")

			err = transform.manager.SetProviders(translation.NewTestProvider(&testdataFiles))
			require.NoError(t, err)
			got, err := transform.processTraces(context.Background(), tt.input())
			if !tt.wantErr(t, err, fmt.Sprintf("processTraces(%v)", tt.input())) {
				return
			}
			targetTrace := got.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			assert.Equal(t, targetTrace.Attributes().Len(), 1)
			_, usedResource := targetTrace.Attributes().Get("one_one_zero_output")
			_, usedScope := targetTrace.Attributes().Get("one_two_zero_output")
			_, usedNoop := targetTrace.Attributes().Get("input")
			switch tt.whichSchemaUsed {
			case ResourceSchemaVersionUsed:
				assert.True(t, usedResource, "processTraces(%v) not using correct schema,, attributes present: %v", tt.name, targetTrace.Attributes().AsRaw())
			case ScopeSchemaVersionUsed:
				assert.True(t, usedScope, "processTraces(%v) not using correct schema, attributes present: %v", tt.name, targetTrace.Attributes().AsRaw())
			case NoopSchemaUsed:
				assert.True(t, usedNoop, "processTraces(%v) not using correct schema,, attributes present: %v", tt.name, targetTrace.Attributes().AsRaw())
			}
		})
	}
}


// returns a test metric with no schema versions set
func GenerateMetricForTest() pmetric.Metrics {
	in := pmetric.NewMetrics()
	in.ResourceMetrics().AppendEmpty()
	in.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
	l := in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
	l.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("input", "test")
	return in
}

// case 1: resource schema set, scope schema not set, use resource schema
// case 2: resource schema not set, scope schema set, use scope schema inside
// case 3: resource schema set, scope schema set, use scope schema
// case 4: resource schema not set, scope schema not set, noop translation
func TestTransformerScopeMetricSchemaPrecedence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   func() pmetric.Metrics
		whichSchemaUsed    SchemaUsed
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "resourcesetscopeunset",
			input: func() pmetric.Metrics {
				metric := GenerateMetricForTest()
				metric.ResourceMetrics().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				return metric
			},
			whichSchemaUsed: ResourceSchemaVersionUsed,
			wantErr: assert.NoError,
		},
		{
			name: "resourceunsetscopeset",
			input: func() pmetric.Metrics {
				metric := GenerateMetricForTest()
				metric.ResourceMetrics().At(0).ScopeMetrics().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")
				return metric
			},
			whichSchemaUsed: ScopeSchemaVersionUsed,
			wantErr: assert.NoError,
		},
		{
			name: "resourcesetscopeset",
			input: func() pmetric.Metrics {
				metric := GenerateMetricForTest()
				metric.ResourceMetrics().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				metric.ResourceMetrics().At(0).ScopeMetrics().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")

				return metric
			},
			whichSchemaUsed: ScopeSchemaVersionUsed,
			wantErr: assert.NoError,
		},
		{
			name: "resourceunsetscopeunset",
			input: func() pmetric.Metrics {
				metric := GenerateMetricForTest()
				return metric
			},
			//want: "https://example.com/testdata/testschemas/schemaprecedence/1.0.0",
			whichSchemaUsed: NoopSchemaUsed,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultConfig := newDefaultConfiguration()
			castedConfig := defaultConfig.(*Config)
			castedConfig.Targets = []string{"https://example.com/testdata/testschemas/schemaprecedence/1.2.0"}
			transform, err := newTransformer(context.Background(), defaultConfig, processor.Settings{
				TelemetrySettings: component.TelemetrySettings{
					Logger: zaptest.NewLogger(t),
				},
			})
			require.NoError(t, err, "Must not error when creating transformer")

			err = transform.manager.SetProviders(translation.NewTestProvider(&testdataFiles))
			require.NoError(t, err)
			got, err := transform.processMetrics(context.Background(), tt.input())
			if !tt.wantErr(t, err, fmt.Sprintf("processMetrics(%v)", tt.input())) {
				return
			}
			targetMetric := got.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0)
			assert.Equal(t, targetMetric.Attributes().Len(), 1)
			_, usedResource := targetMetric.Attributes().Get("one_one_zero_output")
			_, usedScope := targetMetric.Attributes().Get("one_two_zero_output")
			_, usedNoop := targetMetric.Attributes().Get("input")
			switch tt.whichSchemaUsed {
			case ResourceSchemaVersionUsed:
				assert.True(t, usedResource, "processMetrics(%v) not using correct schema,, attributes present: %v", tt.name, targetMetric.Attributes().AsRaw())
			case ScopeSchemaVersionUsed:
				assert.True(t, usedScope, "processMetrics(%v) not using correct schema, attributes present: %v", tt.name, targetMetric.Attributes().AsRaw())
			case NoopSchemaUsed:
				assert.True(t, usedNoop, "processMetrics(%v) not using correct schema,, attributes present: %v", tt.name, targetMetric.Attributes().AsRaw())
			}
		})
	}
}