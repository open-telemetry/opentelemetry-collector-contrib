// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/condition"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadatatest"
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
			ctx := t.Context()
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
				processortest.NewNopSettings(metadata.Type),
				cfg,
				next,
			)
			require.NotNil(t, fmp)
			require.NoError(t, err)

			caps := fmp.Capabilities()
			require.True(t, caps.MutatesData)

			require.NoError(t, fmp.Start(ctx, componenttest.NewNopHost()))

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
		name              string
		conditions        TraceFilters
		contextConditions []condition.ContextConditions
		filterEverything  bool
		want              func(td ptrace.Traces)
		errorMode         ottl.ErrorMode
	}{
		{
			name: "drop resource",
			conditions: TraceFilters{
				ResourceConditions: []string{
					`attributes["host.name"] == "localhost"`,
				},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
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
		{
			name: "with context conditions",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`not IsMatch(span.name, "operation.*")`}},
			},
			want:      func(_ ptrace.Traces) {},
			errorMode: ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Traces: tt.conditions, TraceConditions: tt.contextConditions, ErrorMode: tt.errorMode, spanFunctions: defaultSpanFunctionsMap()}
			processor, err := newFilterSpansProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processTraces(t.Context(), constructTraces())

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
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting
	processor, err := newFilterSpansProcessor(metadatatest.NewSettings(tel), &Config{
		Traces: TraceFilters{
			SpanConditions: []string{
				`name == "operationA"`,
			},
		}, ErrorMode: ottl.IgnoreError,
	})
	assert.NoError(t, err)

	_, err = processor.processTraces(t.Context(), constructTraces())
	assert.NoError(t, err)

	metadatatest.AssertEqualProcessorFilterSpansFiltered(t, tel, []metricdata.DataPoint[int64]{
		{
			Value:      2,
			Attributes: attribute.NewSet(attribute.String("filter", "filter")),
		},
	}, metricdatatest.IgnoreTimestamp())
}

func Test_ProcessTraces_DefinedContext(t *testing.T) {
	tests := []struct {
		name              string
		contextConditions []condition.ContextConditions
		filterEverything  bool
		want              func(td ptrace.Traces)
		errorMode         ottl.ErrorMode
	}{
		{
			name: "resource: drop by schema_url",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`schema_url == "test_schema_url"`}, Context: "resource"},
			},
			want: func(_ ptrace.Traces) {},
		},
		{
			name: "resource: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`attributes["host.name"] == "localhost"`}, Context: "resource"},
			},
			filterEverything: true,
		},
		{
			name: "scope: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`attributes["lib"] == "awesomelib"`}, Context: "scope"},
			},
			want: func(_ ptrace.Traces) {},
		},
		{
			name: "scope: drop by name",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`name == "scope1"`}, Context: "scope"},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
					return ss.Scope().Name() == "scope1"
				})
			},
		},
		{
			name: "span: drop by attributes",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`attributes["total.string"] == "123456789"`}, Context: "span"},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					rs.ScopeSpans().At(i).Spans().RemoveIf(func(span ptrace.Span) bool {
						v, ok := span.Attributes().Get("total.string")
						return ok && v.AsString() == "123456789"
					})
				}
			},
		},
		{
			name: "span: drop by function",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`IsMatch(name, "operationA")`}, Context: "span"},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					rs.ScopeSpans().At(i).Spans().RemoveIf(func(span ptrace.Span) bool {
						return span.Name() == "operationA"
					})
				}
			},
		},
		{
			name: "span: drop by enum",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`status.code == STATUS_CODE_ERROR`}, Context: "span"},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					rs.ScopeSpans().At(i).Spans().RemoveIf(func(span ptrace.Span) bool {
						return span.Status().Code() == ptrace.StatusCodeError
					})
				}
			},
		},
		{
			name: "spanevent: drop by name",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`name == "eventA"`}, Context: "spanevent"},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					for j := 0; j < rs.ScopeSpans().At(i).Spans().Len(); j++ {
						rs.ScopeSpans().At(i).Spans().At(j).Events().RemoveIf(func(event ptrace.SpanEvent) bool {
							return event.Name() == "eventA"
						})
					}
				}
			},
		},
		{
			name: "inferring mixed contexts",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`name == "operationA"`}, Context: "span"},
				{Conditions: []string{`name == "scope1"`}, Context: "scope"},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
					return ss.Scope().Name() == "scope1"
				})
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					rs.ScopeSpans().At(i).Spans().RemoveIf(func(span ptrace.Span) bool {
						return span.Name() == "operationA"
					})
					rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
						return ss.Scope().Name() == "scope1"
					})
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.TraceConditions = tt.contextConditions
			processor, err := newFilterSpansProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processTraces(t.Context(), constructTraces())

			if tt.filterEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructTraces()
				tt.want(exTd)
				assert.Equal(t, exTd, got)
			}
		})
	}
}

func Test_ProcessTraces_InferredContext(t *testing.T) {
	tests := []struct {
		name              string
		contextConditions []condition.ContextConditions
		filterEverything  bool
		want              func(td ptrace.Traces)
		errorMode         ottl.ErrorMode
		input             func() ptrace.Traces
	}{
		{
			name: "resource: drop by schema_url",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`resource.schema_url == "test_schema_url"`}},
			},
			want:  func(_ ptrace.Traces) {},
			input: constructTraces,
		},
		{
			name: "resource: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`resource.attributes["host.name"] == "localhost"`}},
			},
			filterEverything: true,
			input:            constructTraces,
		},
		{
			name: "scope: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`scope.attributes["lib"] == "awesomelib"`}},
			},
			want:  func(_ ptrace.Traces) {},
			input: constructTraces,
		},
		{
			name: "scope: drop by name",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`scope.name == "scope1"`}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
					return ss.Scope().Name() == "scope1"
				})
			},
			input: constructTraces,
		},
		{
			name: "span: drop by attributes",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`span.attributes["total.string"] == "123456789"`}},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					rs.ScopeSpans().At(i).Spans().RemoveIf(func(span ptrace.Span) bool {
						v, ok := span.Attributes().Get("total.string")
						return ok && v.AsString() == "123456789"
					})
				}
			},
			input: constructTraces,
		},
		{
			name: "span: drop by function",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`IsMatch(span.name, "operationA")`}},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					rs.ScopeSpans().At(i).Spans().RemoveIf(func(span ptrace.Span) bool {
						return span.Name() == "operationA"
					})
				}
			},
			input: constructTraces,
		},
		{
			name: "span: drop by enum",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`span.status.code == STATUS_CODE_ERROR`}},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					rs.ScopeSpans().At(i).Spans().RemoveIf(func(span ptrace.Span) bool {
						return span.Status().Code() == ptrace.StatusCodeError
					})
				}
			},
			input: constructTraces,
		},
		{
			name: "spanevent: drop by name",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`spanevent.name == "eventA"`}},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					for j := 0; j < rs.ScopeSpans().At(i).Spans().Len(); j++ {
						rs.ScopeSpans().At(i).Spans().At(j).Events().RemoveIf(func(event ptrace.SpanEvent) bool {
							return event.Name() == "eventA"
						})
					}
				}
			},
			input: constructTraces,
		},
		{
			name: "inferring mixed contexts",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{
					`span.name == "operationA"`,
					`scope.name == "scope1"`,
				}},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
					return ss.Scope().Name() == "scope1"
				})
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					rs.ScopeSpans().At(i).Spans().RemoveIf(func(span ptrace.Span) bool {
						return span.Name() == "operationA"
					})
					rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
						return ss.Scope().Name() == "scope1"
					})
				}
			},
			input: constructTraces,
		},
		{
			name: "zero-record lower-context: resource and spanevent with no events",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{
					`resource.attributes["host.name"] == "localhost"`,
					`spanevent.name == "eventA"`,
				}},
			},
			filterEverything: true,
			input:            contructTracesWithEmptySpanEvent,
		},
		{
			name: "group by context: conditions in same group are respected",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{
					`scope.name == "scope3"`,
					`scope.name == "scope4"`,
					`span.name == "operationA"`,
					`span.name == "operationB"`,
				}},
			},
			filterEverything: true,
			input:            contructTracesWithMultipleSpans,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.TraceConditions = tt.contextConditions
			processor, err := newFilterSpansProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processTraces(t.Context(), tt.input())

			if tt.filterEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := tt.input()
				tt.want(exTd)
				assert.Equal(t, exTd, got)
			}
		})
	}
}

func Test_ProcessTraces_ErrorMode(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "resource",
		},
		{
			name: "scope",
		},
		{
			name: "span",
		},
		{
			name: "spanevent",
		},
	}

	for _, errMode := range []ottl.ErrorMode{ottl.PropagateError, ottl.IgnoreError, ottl.SilentError} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s:%s", tt.name, errMode), func(t *testing.T) {
				cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
				cfg.TraceConditions = []condition.ContextConditions{
					{Conditions: []string{`ParseJSON("1")`}, Context: condition.ContextID(tt.name)},
				}
				cfg.ErrorMode = errMode
				processor, err := newFilterSpansProcessor(processortest.NewNopSettings(metadata.Type), cfg)
				assert.NoError(t, err)

				_, err = processor.processTraces(t.Context(), constructTraces())

				switch errMode {
				case ottl.PropagateError:
					assert.Error(t, err)
				case ottl.IgnoreError, ottl.SilentError:
					assert.NoError(t, err)
				}
			})
		}
	}
}

func Test_ProcessTraces_ConditionsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		conditions    []condition.ContextConditions
		want          func(td ptrace.Traces)
		wantErrorWith string
	}{
		{
			name:      "resource: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("resource")},
				{Conditions: []string{`not IsMatch(resource.attributes["host.name"], ".*")`}, Context: condition.ContextID("resource")},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
					v, _ := rs.Resource().Attributes().Get("host.name")
					return v.AsString() == ""
				})
			},
		},
		{
			name:      "resource: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("resource")},
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("true")`}, Context: condition.ContextID("resource")},
			},
			wantErrorWith: "could not convert parsed value of type bool to JSON object",
		},
		{
			name:      "resource: conditions group error mode with undefined context takes precedence",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
			},
			want: func(_ ptrace.Traces) {},
		},
		{
			name:      "scope: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("scope")},
				{Conditions: []string{`scope.name == "scope1"`}, Context: condition.ContextID("scope")},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
					return ss.Scope().Name() == "scope1"
				})
			},
		},
		{
			name:      "scope: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("scope")},
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON("true")`}, Context: condition.ContextID("scope")},
			},
			wantErrorWith: "could not convert parsed value of type bool to JSON object",
		},
		{
			name:      "span: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`span.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("span")},
				{Conditions: []string{`not IsMatch(span.name, ".*")`}, Context: condition.ContextID("span")},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().RemoveIf(func(span ptrace.Span) bool {
					return span.Name() == ""
				})
			},
		},
		{
			name:      "span: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`span.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("span")},
				{Conditions: []string{`span.attributes["pass"] == ParseJSON("true")`}, Context: condition.ContextID("span")},
			},
			wantErrorWith: "could not convert parsed value of type bool to JSON object",
		},
		{
			name:      "spanevent: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`spanevent.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("spanevent")},
				{Conditions: []string{`spanevent.name == "eventA"`}, Context: condition.ContextID("spanevent")},
			},
			want: func(td ptrace.Traces) {
				rs := td.ResourceSpans().At(0)
				for i := 0; i < rs.ScopeSpans().Len(); i++ {
					for j := 0; j < rs.ScopeSpans().At(i).Spans().Len(); j++ {
						rs.ScopeSpans().At(i).Spans().At(j).Events().RemoveIf(func(event ptrace.SpanEvent) bool {
							return event.Name() == "eventA"
						})
					}
				}
			},
		},
		{
			name:      "spanevent: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`spanevent.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`spanevent.attributes["pass"] == ParseJSON("true")`}},
			},
			wantErrorWith: "could not convert parsed value of type bool to JSON object",
		},
		{
			name:      "flat style propagate error",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{
					Conditions: []string{
						`resource.attributes["pass"] == ParseJSON("1")`,
						`not IsMatch(resource.attributes["host.name"], ".*")`,
					},
				},
			},
			wantErrorWith: "could not convert parsed value of type float64 to JSON object",
		},
		{
			name:      "flat style ignore error",
			errorMode: ottl.IgnoreError,
			conditions: []condition.ContextConditions{
				{
					Conditions: []string{
						`resource.attributes["pass"] == ParseJSON("1")`,
						`not IsMatch(resource.attributes["host.name"], ".*")`,
					},
				},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
					v, _ := rs.Resource().Attributes().Get("host.name")
					return v.AsString() == ""
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.TraceConditions = tt.conditions
			cfg.ErrorMode = tt.errorMode

			processor, err := newFilterSpansProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processTraces(t.Context(), constructTraces())
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}

			assert.NoError(t, err)
			exTd := constructTraces()
			tt.want(exTd)
			assert.Equal(t, exTd, got)
		})
	}
}

type TraceFuncArguments[K any] struct{}

func createTraceFunc[K any](ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(context.Context, K) (any, error) {
		return nil, nil
	}, nil
}

func NewSpanFuncFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TestSpanFunc", &TraceFuncArguments[K]{}, createTraceFunc[K])
}

func NewSpanEventFuncFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TestSpanEventFunc", &TraceFuncArguments[K]{}, createTraceFunc[K])
}

func Test_NewProcessor_NonDefaultFunctions(t *testing.T) {
	type testCase struct {
		name               string
		conditions         []condition.ContextConditions
		wantErrorWith      string
		spanFunctions      map[string]ottl.Factory[*ottlspan.TransformContext]
		spanEventFunctions map[string]ottl.Factory[*ottlspanevent.TransformContext]
	}

	tests := []testCase{
		{
			name: "span functions : statement with added span func",
			conditions: []condition.ContextConditions{
				{
					Context:    condition.ContextID("span"),
					Conditions: []string{`IsMatch(name, TestSpanFunc())`},
				},
			},
			spanFunctions: map[string]ottl.Factory[*ottlspan.TransformContext]{
				"IsMatch":      defaultSpanFunctionsMap()["IsMatch"],
				"TestSpanFunc": NewSpanFuncFactory[*ottlspan.TransformContext](),
			},
			spanEventFunctions: defaultSpanEventFunctionsMap(),
		},
		{
			name: "span functions : statement with missing span func",
			conditions: []condition.ContextConditions{
				{
					Context:    condition.ContextID("span"),
					Conditions: []string{`IsMatch(name, TestSpanFunc())`},
				},
			},
			wantErrorWith:      `undefined function "TestSpanFunc"`,
			spanFunctions:      defaultSpanFunctionsMap(),
			spanEventFunctions: defaultSpanEventFunctionsMap(),
		},
		{
			name: "span event functions : statement with added span event func",
			conditions: []condition.ContextConditions{
				{
					Context:    condition.ContextID("spanevent"),
					Conditions: []string{`IsMatch(name, TestSpanFunc())`},
				},
			},
			spanFunctions: defaultSpanFunctionsMap(),
			spanEventFunctions: map[string]ottl.Factory[*ottlspanevent.TransformContext]{
				"IsMatch":      defaultSpanEventFunctionsMap()["IsMatch"],
				"TestSpanFunc": NewSpanEventFuncFactory[*ottlspanevent.TransformContext](),
			},
		},
		{
			name: "span event functions : statement with missing span event func",
			conditions: []condition.ContextConditions{
				{
					Context:    condition.ContextID("spanevent"),
					Conditions: []string{`IsMatch(name, TestSpanEventFunc())`},
				},
			},
			wantErrorWith:      `undefined function "TestSpanEventFunc"`,
			spanFunctions:      defaultSpanFunctionsMap(),
			spanEventFunctions: defaultSpanEventFunctionsMap(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.TraceConditions = tt.conditions
			cfg.spanFunctions = tt.spanFunctions
			cfg.spanEventFunctions = tt.spanEventFunctions

			_, err := newFilterSpansProcessor(processortest.NewNopSettings(metadata.Type), cfg)

			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
		})
	}
}

func contructTracesWithEmptySpanEvent() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("host.name", "localhost")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("scope1")
	span := ss.Spans().AppendEmpty()
	span.SetName("operationA")
	return td
}

func contructTracesWithMultipleSpans() ptrace.Traces {
	td := ptrace.NewTraces()
	rs0 := td.ResourceSpans().AppendEmpty()
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeSpans().AppendEmpty()
	rs0ils0.Scope().SetName("scope1")
	fillSpanOne(rs0ils0.Spans().AppendEmpty())
	fillSpanOne(rs0ils0.Spans().AppendEmpty())
	rs0ils1 := rs0.ScopeSpans().AppendEmpty()
	rs0ils1.Scope().SetName("scope2")
	fillSpanTwo(rs0ils1.Spans().AppendEmpty())
	fillSpanTwo(rs0ils1.Spans().AppendEmpty())
	rs0ils3 := rs0.ScopeSpans().AppendEmpty()
	rs0ils3.Scope().SetName("scope3")
	rs0ils4 := rs0.ScopeSpans().AppendEmpty()
	rs0ils4.Scope().SetName("scope4")
	return td
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
	span.Attributes().PutStr("total.string", "123456789")
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
	spanEvent := span.Events().AppendEmpty()
	spanEvent.SetName("eventA")
}

func fillSpanTwo(span ptrace.Span) {
	span.SetName("operationB")
	span.SetStartTimestamp(testSpanStartTimestamp)
	span.SetEndTimestamp(testSpanEndTimestamp)
	span.Attributes().PutStr("http.method", "get")
	span.Attributes().PutStr("http.path", "/health")
	span.Attributes().PutStr("http.url", "http://localhost/health")
	span.Attributes().PutStr("flags", "C|D")
	span.Attributes().PutStr("total.string", "345678")
	link0 := span.Links().AppendEmpty()
	link0.SetDroppedAttributesCount(4)
	link1 := span.Links().AppendEmpty()
	link1.SetDroppedAttributesCount(4)
	span.SetDroppedLinksCount(3)
	status := span.Status()
	status.SetCode(ptrace.StatusCodeOk)
	status.SetMessage("status-cancelled")
	spanEvent := span.Events().AppendEmpty()
	spanEvent.SetName("spanEventA")
}
