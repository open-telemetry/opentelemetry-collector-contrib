// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// All the data we need to test the Span filter
type testTrace struct {
	spanName           string
	libraryName        string
	libraryVersion     string
	resourceAttributes map[string]any
	tags               map[string]any
}

// All the data we need to define a test
type traceTest struct {
	name              string
	inc               *filterconfig.MatchProperties
	exc               *filterconfig.MatchProperties
	inTraces          ptrace.Traces
	allTracesFiltered bool
	spanCountExpected int // The number of spans that should be left after all filtering
}

var (
	redisTraces = []testTrace{
		{
			spanName:       "test!",
			libraryName:    "otel",
			libraryVersion: "11",
			resourceAttributes: map[string]any{
				"service.name": "test_service",
			},
			tags: map[string]any{
				"db.type": "redis",
			},
		},
	}

	nameTraces = []testTrace{
		{
			spanName:       "test!",
			libraryName:    "otel",
			libraryVersion: "11",
			resourceAttributes: map[string]any{
				"service.name": "keep",
			},
		},
		{
			spanName:       "test!",
			libraryName:    "otel",
			libraryVersion: "11",
			resourceAttributes: map[string]any{
				"service.name": "dont_keep",
			},
		},
		{
			spanName:       "test!",
			libraryName:    "otel",
			libraryVersion: "11",
			resourceAttributes: map[string]any{
				"service.name": "keep",
			},
		},
	}

	serviceNameMatchProperties = &filterconfig.MatchProperties{
		Config:   filterset.Config{MatchType: filterset.Strict},
		Services: []string{"keep"},
	}

	redisMatchProperties = &filterconfig.MatchProperties{
		Config: filterset.Config{MatchType: filterset.Strict},
		Attributes: []filterconfig.Attribute{
			{Key: "db.type", Value: "redis"},
		},
	}

	standardTraceTests = []traceTest{
		{
			name:              "filterRedis",
			exc:               redisMatchProperties,
			inTraces:          generateTraces(redisTraces),
			allTracesFiltered: true,
		},
		{
			name:              "keepRedis",
			inc:               redisMatchProperties,
			inTraces:          generateTraces(redisTraces),
			spanCountExpected: 1,
		},
		{
			name:              "keepServiceName",
			inc:               serviceNameMatchProperties,
			inTraces:          generateTraces(nameTraces),
			spanCountExpected: 2,
		},
	}
)

func TestFilterTraceProcessor(t *testing.T) {
	for _, test := range standardTraceTests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			next := new(consumertest.TracesSink)
			cfg := &Config{
				Spans: filterconfig.MatchConfig{
					Include: test.inc,
					Exclude: test.exc,
				},
			}
			factory := NewFactory()
			fmp, err := factory.CreateTraces(
				ctx,
				processortest.NewNopSettings(),
				cfg,
				next,
			)
			require.NotNil(t, fmp)
			require.NoError(t, err)

			caps := fmp.Capabilities()
			require.True(t, caps.MutatesData)

			require.NoError(t, fmp.Start(ctx, nil))

			cErr := fmp.ConsumeTraces(ctx, test.inTraces)
			require.NoError(t, cErr)
			got := next.AllTraces()

			// If all traces got filtered you shouldn't even have ResourceSpans
			if test.allTracesFiltered {
				require.Empty(t, got)
			} else {
				require.Equal(t, test.spanCountExpected, got[0].SpanCount())
			}
			require.NoError(t, fmp.Shutdown(ctx))
		})
	}
}

func generateTraces(traces []testTrace) ptrace.Traces {
	td := ptrace.NewTraces()

	for _, trace := range traces {
		rs := td.ResourceSpans().AppendEmpty()
		//nolint:errcheck
		rs.Resource().Attributes().FromRaw(trace.resourceAttributes)
		ils := rs.ScopeSpans().AppendEmpty()
		ils.Scope().SetName(trace.libraryName)
		ils.Scope().SetVersion(trace.libraryVersion)
		span := ils.Spans().AppendEmpty()
		//nolint:errcheck
		span.Attributes().FromRaw(trace.tags)
		span.SetName(trace.spanName)
	}
	return td
}

var (
	testSpanStartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	testSpanStartTimestamp = pcommon.NewTimestampFromTime(testSpanStartTime)

	testSpanEndTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	testSpanEndTimestamp = pcommon.NewTimestampFromTime(testSpanEndTime)

	traceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	spanID2 = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func TestFilterTraceProcessorWithOTTL(t *testing.T) {
	tests := []struct {
		name             string
		conditions       TraceFilters
		filterEverything bool
		want             func(td ptrace.Traces)
		errorMode        ottl.ErrorMode
	}{
		{
			name: "drop spans",
			conditions: TraceFilters{
				SpanConditions: []string{
					`name == "operationA"`,
				},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().RemoveIf(func(span ptrace.Span) bool {
					return span.Name() == "operationA"
				})
				td.ResourceSpans().At(0).ScopeSpans().At(1).Spans().RemoveIf(func(span ptrace.Span) bool {
					return span.Name() == "operationA"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop everything by dropping all spans",
			conditions: TraceFilters{
				SpanConditions: []string{
					`IsMatch(name, "operation.*")`,
				},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "drop span events",
			conditions: TraceFilters{
				SpanEventConditions: []string{
					`name == "spanEventA"`,
				},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Events().RemoveIf(func(event ptrace.SpanEvent) bool {
					return event.Name() == "spanEventA"
				})
				td.ResourceSpans().At(0).ScopeSpans().At(1).Spans().At(1).Events().RemoveIf(func(event ptrace.SpanEvent) bool {
					return event.Name() == "spanEventA"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "multiple conditions",
			conditions: TraceFilters{
				SpanConditions: []string{
					`name == "operationZ"`,
					`span_id != nil`,
				},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "with error conditions",
			conditions: TraceFilters{
				SpanConditions: []string{
					`Substring("", 0, 100) == "test"`,
				},
			},
			want:      func(_ ptrace.Traces) {},
			errorMode: ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := newFilterSpansProcessor(processortest.NewNopSettings(), &Config{Traces: tt.conditions, ErrorMode: tt.errorMode})
			assert.NoError(t, err)

			got, err := processor.processTraces(context.Background(), constructTraces())

			if tt.filterEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {

				exTd := constructTraces()
				tt.want(exTd)
				assert.Equal(t, exTd, got)
			}
		})
	}
}

func TestFilterTraceProcessorTelemetry(t *testing.T) {
	tel := setupTestTelemetry()
	processor, err := newFilterSpansProcessor(tel.NewSettings(), &Config{
		Traces: TraceFilters{
			SpanConditions: []string{
				`name == "operationA"`,
			},
		}, ErrorMode: ottl.IgnoreError,
	})
	assert.NoError(t, err)

	_, err = processor.processTraces(context.Background(), constructTraces())
	assert.NoError(t, err)

	want := []metricdata.Metrics{
		{
			Name:        "otelcol_processor_filter_spans.filtered",
			Description: "Number of spans dropped by the filter processor",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value:      2,
						Attributes: attribute.NewSet(attribute.String("filter", "filter")),
					},
				},
			},
		},
	}

	tel.assertMetrics(t, want)
}

func constructTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs0 := td.ResourceSpans().AppendEmpty()
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeSpans().AppendEmpty()
	rs0ils0.Scope().SetName("scope1")
	fillSpanOne(rs0ils0.Spans().AppendEmpty())
	fillSpanTwo(rs0ils0.Spans().AppendEmpty())
	rs0ils1 := rs0.ScopeSpans().AppendEmpty()
	rs0ils1.Scope().SetName("scope2")
	fillSpanOne(rs0ils1.Spans().AppendEmpty())
	fillSpanTwo(rs0ils1.Spans().AppendEmpty())
	return td
}

func fillSpanOne(span ptrace.Span) {
	span.SetName("operationA")
	span.SetSpanID(spanID)
	span.SetParentSpanID(spanID2)
	span.SetTraceID(traceID)
	span.SetStartTimestamp(testSpanStartTimestamp)
	span.SetEndTimestamp(testSpanEndTimestamp)
	span.SetDroppedAttributesCount(1)
	span.SetDroppedLinksCount(1)
	span.SetDroppedEventsCount(1)
	span.SetKind(1)
	span.TraceState().FromRaw("new")
	span.Attributes().PutStr("http.method", "get")
	span.Attributes().PutStr("http.path", "/health")
	span.Attributes().PutStr("http.url", "http://localhost/health")
	span.Attributes().PutStr("flags", "A|B|C")
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
}

func fillSpanTwo(span ptrace.Span) {
	span.SetName("operationB")
	span.SetStartTimestamp(testSpanStartTimestamp)
	span.SetEndTimestamp(testSpanEndTimestamp)
	span.Attributes().PutStr("http.method", "get")
	span.Attributes().PutStr("http.path", "/health")
	span.Attributes().PutStr("http.url", "http://localhost/health")
	span.Attributes().PutStr("flags", "C|D")
	link0 := span.Links().AppendEmpty()
	link0.SetDroppedAttributesCount(4)
	link1 := span.Links().AppendEmpty()
	link1.SetDroppedAttributesCount(4)
	span.SetDroppedLinksCount(3)
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
	spanEvent := span.Events().AppendEmpty()
	spanEvent.SetName("spanEventA")
}
