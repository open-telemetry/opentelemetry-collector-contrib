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
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"
)

func newTestTransformer(t *testing.T) *transformer {
	trans, err := newTransformer(context.Background(), newDefaultConfiguration(), processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger:        zaptest.NewLogger(t),
			MeterProvider: noop.MeterProvider{},
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

//go:embed testdata
var f embed.FS

func plogsFromJSON(t *testing.T, path string) plog.Logs {
	t.Helper()
	unmarshaler := plog.JSONUnmarshaler{}
	signalJSON, err := f.ReadFile(path)
	require.NoError(t, err)
	inSignals, err := unmarshaler.UnmarshalLogs(signalJSON)
	require.NoError(t, err)
	require.NotNil(t, inSignals.ResourceLogs())
	require.NotEqual(t, 0, inSignals.LogRecordCount())
	return inSignals
}

func ptracesFromJSON(t *testing.T, path string) ptrace.Traces {
	t.Helper()
	unmarshaler := ptrace.JSONUnmarshaler{}
	signalJSON, err := f.ReadFile(path)
	require.NoError(t, err)
	inSignals, err := unmarshaler.UnmarshalTraces(signalJSON)
	require.NoError(t, err)
	require.NotNil(t, inSignals.ResourceSpans())
	require.NotEqual(t, 0, inSignals.SpanCount())
	return inSignals
}

func pmetricsFromJSON(t *testing.T, path string) pmetric.Metrics {
	t.Helper()
	unmarshaler := pmetric.JSONUnmarshaler{}
	signalJSON, err := f.ReadFile(path)
	require.NoError(t, err)
	inSignals, err := unmarshaler.UnmarshalMetrics(signalJSON)
	require.NoError(t, err)
	require.NotNil(t, inSignals.ResourceMetrics())
	require.NotEqual(t, 0, inSignals.MetricCount())
	return inSignals
}

func buildTestTransformer(t *testing.T, targetURL string) *transformer {
	t.Helper()
	defaultConfig := newDefaultConfiguration()
	castedConfig := defaultConfig.(*Config)
	castedConfig.Targets = []string{targetURL}
	transform, err := newTransformer(context.Background(), castedConfig, processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zaptest.NewLogger(t),
		},
	})
	require.NoError(t, err, "Must not error when creating transformer")
	err = transform.manager.SetProviders(translation.NewTestProvider(&f))
	require.NoError(t, err)
	return transform
}

func TestTransformerSchemaBySections(t *testing.T) {
	tests := []struct {
		name     string
		section  string
		dataType pipeline.Signal
	}{
		{
			// todo(ankit) do i need to test all data types here?
			name:     "all_logs",
			section:  "all",
			dataType: pipeline.SignalLogs,
		},
		{
			// todo(ankit) do i need to test all data types here?
			name:     "resources_logs",
			section:  "resources",
			dataType: pipeline.SignalLogs,
		},
		{
			name:     "spans",
			section:  "spans",
			dataType: pipeline.SignalTraces,
		},
		{
			name:     "span_events_rename_spans",
			section:  "span_events_rename_spans",
			dataType: pipeline.SignalTraces,
		},
		{
			name:     "span_events_rename_attributes",
			section:  "span_events_rename_attributes",
			dataType: pipeline.SignalTraces,
		},
		{
			name:     "metrics_rename_metrics",
			section:  "metrics_rename_metrics",
			dataType: pipeline.SignalMetrics,
		},
		{
			name:     "metrics_rename_attributes",
			section:  "metrics_rename_attributes",
			dataType: pipeline.SignalMetrics,
		},
		{
			name:     "logs_rename_attributes",
			section:  "logs_rename_attributes",
			dataType: pipeline.SignalLogs,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			transformerTarget := fmt.Sprintf("https://example.com/testdata/schema_sections/%s/1.0.0", tc.section)
			transform := buildTestTransformer(t, transformerTarget)
			inDataPath := fmt.Sprintf("testdata/schema_sections/%s/%s_in.json", tc.section, tc.dataType)
			outDataPath := fmt.Sprintf("testdata/schema_sections/%s/%s_out.json", tc.section, tc.dataType)
			switch tc.dataType {
			case pipeline.SignalLogs:
				inLogs := plogsFromJSON(t, inDataPath)
				expected := plogsFromJSON(t, outDataPath)

				logs, err := transform.processLogs(context.Background(), inLogs)
				require.NoError(t, err)
				require.NoError(t, plogtest.CompareLogs(expected, logs), "Must match the expected values")
			case pipeline.SignalMetrics:
				inMetrics := pmetricsFromJSON(t, inDataPath)
				expected := pmetricsFromJSON(t, outDataPath)

				metrics, err := transform.processMetrics(context.Background(), inMetrics)
				require.NoError(t, err)
				require.NoError(t, pmetrictest.CompareMetrics(expected, metrics), "Must match the expected values")
			case pipeline.SignalTraces:
				inTraces := ptracesFromJSON(t, inDataPath)
				expected := ptracesFromJSON(t, outDataPath)

				traces, err := transform.processTraces(context.Background(), inTraces)
				require.NoError(t, err)
				require.NoError(t, ptracetest.CompareTraces(expected, traces), "Must match the expected values")
			default:
				require.FailNow(t, "unrecognized data type")
				return
			}
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
func generateLogForTest() plog.Logs {
	in := plog.NewLogs()
	in.ResourceLogs().AppendEmpty()
	in.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
	l := in.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
	l.Attributes().PutStr("input", "test")
	return in
}

//go:embed testdata/testschemas
var testdataFiles embed.FS

// case 1: resource schema set, scope schema not set, use resource schema
// case 2: resource schema not set, scope schema set, use scope schema inside
// case 3: resource schema set, scope schema set, use scope schema
// case 4: resource schema not set, scope schema not set, noop translation
func TestTransformerScopeLogSchemaPrecedence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		input           func() plog.Logs
		whichSchemaUsed SchemaUsed
		wantErr         assert.ErrorAssertionFunc
	}{
		{
			name: "resourcesetscopeunset",
			input: func() plog.Logs {
				log := generateLogForTest()
				log.ResourceLogs().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				return log
			},
			whichSchemaUsed: ResourceSchemaVersionUsed,
			wantErr:         assert.NoError,
		},
		{
			name: "resourceunsetscopeset",
			input: func() plog.Logs {
				log := generateLogForTest()
				log.ResourceLogs().At(0).ScopeLogs().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")
				return log
			},
			whichSchemaUsed: ScopeSchemaVersionUsed,
			wantErr:         assert.NoError,
		},
		{
			name: "resourcesetscopeset",
			input: func() plog.Logs {
				log := generateLogForTest()
				log.ResourceLogs().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				log.ResourceLogs().At(0).ScopeLogs().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")

				return log
			},
			whichSchemaUsed: ScopeSchemaVersionUsed,
			wantErr:         assert.NoError,
		},
		{
			name: "resourceunsetscopeunset",
			input: func() plog.Logs {
				log := generateLogForTest()
				return log
			},
			// want: "https://example.com/testdata/testschemas/schemaprecedence/1.0.0",
			whichSchemaUsed: NoopSchemaUsed,
			wantErr:         assert.NoError,
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
			assert.Equal(t, 1, targetLog.Attributes().Len())
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
func generateTraceForTest() ptrace.Traces {
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
		name            string
		input           func() ptrace.Traces
		whichSchemaUsed SchemaUsed
		wantErr         assert.ErrorAssertionFunc
	}{
		{
			name: "resourcesetscopeunset",
			input: func() ptrace.Traces {
				trace := generateTraceForTest()
				trace.ResourceSpans().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				return trace
			},
			whichSchemaUsed: ResourceSchemaVersionUsed,
			wantErr:         assert.NoError,
		},
		{
			name: "resourceunsetscopeset",
			input: func() ptrace.Traces {
				trace := generateTraceForTest()
				trace.ResourceSpans().At(0).ScopeSpans().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")
				return trace
			},
			whichSchemaUsed: ScopeSchemaVersionUsed,
			wantErr:         assert.NoError,
		},
		{
			name: "resourcesetscopeset",
			input: func() ptrace.Traces {
				trace := generateTraceForTest()
				trace.ResourceSpans().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				trace.ResourceSpans().At(0).ScopeSpans().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")

				return trace
			},
			whichSchemaUsed: ScopeSchemaVersionUsed,
			wantErr:         assert.NoError,
		},
		{
			name: "resourceunsetscopeunset",
			input: func() ptrace.Traces {
				trace := generateTraceForTest()
				return trace
			},
			// want: "https://example.com/testdata/testschemas/schemaprecedence/1.0.0",
			whichSchemaUsed: NoopSchemaUsed,
			wantErr:         assert.NoError,
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
			assert.Equal(t, 1, targetTrace.Attributes().Len())
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
func generateMetricForTest() pmetric.Metrics {
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
		name            string
		input           func() pmetric.Metrics
		whichSchemaUsed SchemaUsed
		wantErr         assert.ErrorAssertionFunc
	}{
		{
			name: "resourcesetscopeunset",
			input: func() pmetric.Metrics {
				metric := generateMetricForTest()
				metric.ResourceMetrics().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				return metric
			},
			whichSchemaUsed: ResourceSchemaVersionUsed,
			wantErr:         assert.NoError,
		},
		{
			name: "resourceunsetscopeset",
			input: func() pmetric.Metrics {
				metric := generateMetricForTest()
				metric.ResourceMetrics().At(0).ScopeMetrics().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")
				return metric
			},
			whichSchemaUsed: ScopeSchemaVersionUsed,
			wantErr:         assert.NoError,
		},
		{
			name: "resourcesetscopeset",
			input: func() pmetric.Metrics {
				metric := generateMetricForTest()
				metric.ResourceMetrics().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.0.0")
				metric.ResourceMetrics().At(0).ScopeMetrics().At(0).SetSchemaUrl("https://example.com/testdata/testschemas/schemaprecedence/1.1.0")

				return metric
			},
			whichSchemaUsed: ScopeSchemaVersionUsed,
			wantErr:         assert.NoError,
		},
		{
			name: "resourceunsetscopeunset",
			input: func() pmetric.Metrics {
				metric := generateMetricForTest()
				return metric
			},
			// want: "https://example.com/testdata/testschemas/schemaprecedence/1.0.0",
			whichSchemaUsed: NoopSchemaUsed,
			wantErr:         assert.NoError,
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
			assert.Equal(t, 1, targetMetric.Attributes().Len())
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
