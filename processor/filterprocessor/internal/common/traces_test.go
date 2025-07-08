// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common_test // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common_test"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"
)

var (
	TestSpanStartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestSpanStartTimestamp = pcommon.NewTimestampFromTime(TestSpanStartTime)

	TestSpanEndTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestSpanEndTimestamp = pcommon.NewTimestampFromTime(TestSpanEndTime)

	spanID2 = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func TestFilterTraceProcessorWithOTTL(t *testing.T) {
	tests := []struct {
		name             string
		conditions       []string
		filterEverything bool
		want             func(td ptrace.Traces)
		wantErr          bool
		errorMode        ottl.ErrorMode
	}{
		{
			name: "drop spans",
			conditions: []string{
				`span.name == "operationA"`,
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().RemoveIf(func(span ptrace.Span) bool {
					return span.Name() == "operationA"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop everything by dropping all spans",
			conditions: []string{
				`IsMatch(span.name, "operation.*")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "drop span events",
			conditions: []string{
				`spanevent.name == "spanEventA"`,
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Events().RemoveIf(func(event ptrace.SpanEvent) bool {
					return event.Name() == "spanEventA"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`span.name == "operationZ"`,
				`span.span_id != nil`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "filters resource",
			conditions: []string{
				`resource.schema_url == "test_schema_url"`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "with error conditions",
			conditions: []string{
				`Substring("", 0, 100) == "test"`,
			},
			want:      func(_ ptrace.Traces) {},
			wantErr:   true,
			errorMode: ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection, err := common.NewTraceParserCollection(componenttest.NewNopTelemetrySettings(), common.WithSpanParser(filterottl.StandardSpanFuncs()), common.WithSpanEventParser(filterottl.StandardSpanEventFuncs()))
			assert.NoError(t, err)
			got, err := collection.ParseContextConditions(common.ContextConditions{Conditions: tt.conditions, ErrorMode: tt.errorMode})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err, "error parsing conditions")
			finalTraces := constructTraces()
			consumeErr := got.ConsumeTraces(context.Background(), finalTraces)
			switch {
			case tt.filterEverything && !tt.wantErr:
				assert.Equal(t, processorhelper.ErrSkipProcessingData, consumeErr)
			case tt.wantErr:
				assert.Error(t, consumeErr)
			default:
				assert.NoError(t, consumeErr)
				exTd := constructTraces()
				tt.want(exTd)
				assert.Equal(t, exTd, finalTraces)
			}
		})
	}
}

func Test_ProcessTraces_ConditionsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		conditions    []common.ContextConditions
		want          func(td ptrace.Traces)
		wantErr       bool
		wantErrorWith string
	}{
		{
			name:      "span: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`span.attributes["pass"] == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`not IsMatch(span.name, ".*")`}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().RemoveIf(func(span ptrace.Span) bool {
					return len(span.Name()) == 0
				})
			},
		},
		{
			name:      "span: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`span.attributes["pass"] == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`span.attributes["pass"] == ParseJSON(true)`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "resource: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`not IsMatch(resource.attributes["host.name"], ".*")`}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
					v, _ := rs.Resource().Attributes().Get("host.name")
					return len(v.AsString()) == 0
				})
			},
		},
		{
			name:      "resource: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON(true)`}},
			},
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "scope: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`scope.schema_url != "test_schema_url"`}},
			},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
					return ss.SchemaUrl() != "test_schema_url"
				})
			},
		},
		{
			name:      "scope: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON(1)`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON(true)`}},
			},
			wantErrorWith: "expected string but got bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			collection, err := common.NewTraceParserCollection(componenttest.NewNopTelemetrySettings(), common.WithSpanParser(filterottl.StandardSpanFuncs()), common.WithSpanEventParser(filterottl.StandardSpanEventFuncs()), common.WithTraceErrorMode(tt.errorMode))
			assert.NoError(t, err)

			var consumers []common.TracesConsumer
			for _, condition := range tt.conditions {
				consumer, err := collection.ParseContextConditions(condition)
				if tt.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err, "error parsing conditions")
				consumers = append(consumers, consumer)
			}

			finalTraces := constructTraces()
			var consumeErr error

			// Apply each consumer sequentially
			for _, consumer := range consumers {
				if err := consumer.ConsumeTraces(context.Background(), finalTraces); err != nil {
					if err == processorhelper.ErrSkipProcessingData {
						consumeErr = err
						break
					}
					consumeErr = err
					break
				}
			}

			if tt.wantErrorWith != "" {
				if consumeErr == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				} else {
					assert.Contains(t, consumeErr.Error(), tt.wantErrorWith)
				}
				return
			}

			if consumeErr != nil && consumeErr != processorhelper.ErrSkipProcessingData {
				assert.NoError(t, consumeErr)
				return
			}

			exTd := constructTraces()
			tt.want(exTd)
			assert.Equal(t, exTd, finalTraces)
		})
	}
}

func Test_ProcessTraces_InferredResourceContext(t *testing.T) {
	tests := []struct {
		condition          string
		filteredEverything bool
		want               func(td ptrace.Traces)
	}{
		{
			condition:          `resource.attributes["host.name"] == "localhost"`,
			filteredEverything: true,
			want: func(_ ptrace.Traces) {
				// Everything should be filtered out
			},
		},
		{
			condition:          `resource.attributes["host.name"] == "wrong"`,
			filteredEverything: false,
			want: func(td ptrace.Traces) {
				// Nothing should be filtered, original data remains
			},
		},
		{
			condition:          `resource.schema_url == "test_schema_url"`,
			filteredEverything: true,
			want: func(_ ptrace.Traces) {
				// Everything should be filtered out since schema_url matches
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			td := constructTraces()

			collection, err := common.NewTraceParserCollection(componenttest.NewNopTelemetrySettings(), common.WithSpanParser(filterottl.StandardSpanFuncs()), common.WithSpanEventParser(filterottl.StandardSpanEventFuncs()))
			assert.NoError(t, err)

			consumer, err := collection.ParseContextConditions(common.ContextConditions{Conditions: []string{tt.condition}})
			assert.NoError(t, err)

			err = consumer.ConsumeTraces(context.Background(), td)

			if tt.filteredEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructTraces()
				tt.want(exTd)
				assert.Equal(t, exTd, td)
			}
		})
	}
}

func Test_ProcessTraces_InferredScopeContext(t *testing.T) {
	tests := []struct {
		condition          string
		filteredEverything bool
		want               func(td ptrace.Traces)
	}{
		{
			condition:          `scope.name == "scope"`,
			filteredEverything: true,
			want: func(_ ptrace.Traces) {
				// Everything should be filtered out since scope name matches
			},
		},
		{
			condition:          `scope.version == "2"`,
			filteredEverything: false,
			want: func(td ptrace.Traces) {
				// Nothing should be filtered, original data remains
			},
		},
		{
			condition:          `scope.schema_url == "test_schema_url"`,
			filteredEverything: true,
			want: func(_ ptrace.Traces) {
				// Everything should be filtered out since schema_url matches
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			td := constructTraces()

			collection, err := common.NewTraceParserCollection(componenttest.NewNopTelemetrySettings(), common.WithSpanParser(filterottl.StandardSpanFuncs()), common.WithSpanEventParser(filterottl.StandardSpanEventFuncs()))
			assert.NoError(t, err)

			consumer, err := collection.ParseContextConditions(common.ContextConditions{Conditions: []string{tt.condition}})
			assert.NoError(t, err)

			err = consumer.ConsumeTraces(context.Background(), td)

			if tt.filteredEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructTraces()
				tt.want(exTd)
				assert.Equal(t, exTd, td)
			}
		})
	}
}

func constructTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs0 := td.ResourceSpans().AppendEmpty()
	rs0.SetSchemaUrl("test_schema_url")
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeSpans().AppendEmpty()
	rs0ils0.SetSchemaUrl("test_schema_url")
	rs0ils0.Scope().SetName("scope")
	fillSpanOne(rs0ils0.Spans().AppendEmpty())
	fillSpanTwo(rs0ils0.Spans().AppendEmpty())
	return td
}

func fillSpanOne(span ptrace.Span) {
	span.SetName("operationA")
	span.SetSpanID(spanID)
	span.SetParentSpanID(spanID2)
	span.SetTraceID(traceID)
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
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
	event := span.Events().AppendEmpty()
	event.SetName("eventA")
}

func fillSpanTwo(span ptrace.Span) {
	span.SetName("operationB")
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
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
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
	event := span.Events().AppendEmpty()
	event.SetName("eventB")
}
