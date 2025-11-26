// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package contextfilter_test // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/contextfilter_test"

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	common "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/contextfilter"
)

var (
	TestLogTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestLogTimestamp = pcommon.NewTimestampFromTime(TestLogTime)

	TestObservedTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestObservedTimestamp = pcommon.NewTimestampFromTime(TestObservedTime)

	traceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
)

func TestFilterLogProcessorWithOTTL(t *testing.T) {
	tests := []struct {
		name             string
		conditions       []string
		filterEverything bool
		want             func(ld plog.Logs)
		wantErr          bool
		errorMode        ottl.ErrorMode
	}{
		{
			name: "drop logs",
			conditions: []string{
				`log.body == "operationA"`,
			},
			want: func(ld plog.Logs) {
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
					return log.Body().AsString() == "operationA"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop everything by dropping all logs",
			conditions: []string{
				`IsMatch(log.body, "operation.*")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`IsMatch(log.body, "wrong name")`,
				`IsMatch(log.body, "operation.*")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "with error conditions",
			conditions: []string{
				`Substring("", 0, 100) == "test"`,
			},
			want:      func(_ plog.Logs) {},
			wantErr:   true,
			errorMode: ottl.IgnoreError,
		},
		{
			name: "filters resource",
			conditions: []string{
				`resource.schema_url == "test_schema_url"`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection, err := common.NewLogParserCollection(componenttest.NewNopTelemetrySettings(), common.WithLogParser(filterottl.StandardLogFuncs()))
			assert.NoError(t, err)
			got, err := collection.ParseContextConditions(common.ContextConditions{Conditions: tt.conditions, ErrorMode: tt.errorMode})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err, "error parsing conditions")
			finalLogs := constructLogs()
			consumeErr := got.ConsumeLogs(t.Context(), finalLogs)
			switch {
			case tt.filterEverything && !tt.wantErr:
				assert.Equal(t, processorhelper.ErrSkipProcessingData, consumeErr)
			case tt.wantErr:
				assert.Error(t, consumeErr)
			default:
				assert.NoError(t, consumeErr)
				exTd := constructLogs()
				tt.want(exTd)
				assert.Equal(t, exTd, finalLogs)
			}
		})
	}
}

func Test_ProcessLogs_ConditionsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		conditions    []common.ContextConditions
		want          func(td plog.Logs)
		wantErr       bool
		wantErrorWith string
	}{
		{
			name:      "log: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`log.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`not IsMatch(log.body, ".*")`}},
			},
			want: func(ld plog.Logs) {
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
					return log.Body().AsString() == ""
				})
			},
		},
		{
			name:      "log: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`log.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`log.attributes["pass"] == ParseJSON(true)`}},
			},
			wantErr:       true,
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "resource: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`not IsMatch(resource.attributes["host.name"], ".*")`}},
			},
			want: func(ld plog.Logs) {
				ld.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
					v, _ := rl.Resource().Attributes().Get("host.name")
					return v.AsString() == ""
				})
			},
		},
		{
			name:      "resource: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON(true)`}},
			},
			wantErr:       true,
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "scope: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`scope.schema_url != "test_schema_url"`}},
			},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
					return sl.SchemaUrl() != "test_schema_url"
				})
			},
		},
		{
			name:      "scope: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON(true)`}},
			},
			wantErr:       true,
			wantErrorWith: "expected string but got bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection, err := common.NewLogParserCollection(componenttest.NewNopTelemetrySettings(), common.WithLogParser(filterottl.StandardLogFuncs()), common.WithLogErrorMode(tt.errorMode))
			assert.NoError(t, err)

			var consumers []common.LogsConsumer
			var parseErrs []error
			for _, condition := range tt.conditions {
				consumer, err := collection.ParseContextConditions(condition)
				parseErrs = append(parseErrs, err)
				consumers = append(consumers, consumer)
			}

			if tt.wantErr {
				found := false
				for _, e := range parseErrs {
					if e != nil && strings.Contains(e.Error(), tt.wantErrorWith) {
						found = true
						break
					}
				}
				assert.True(t, found, "expected error containing '%s' but none found", tt.wantErrorWith)
				return
			}

			for _, e := range parseErrs {
				assert.NoError(t, e, "error parsing conditions")
			}

			finalLogs := constructLogs()

			for _, consumer := range consumers {
				if err := consumer.ConsumeLogs(t.Context(), finalLogs); err != nil {
					// ErrSkipProcessingData is expected behavior, continue to validate
					if errors.Is(err, processorhelper.ErrSkipProcessingData) {
						break
					}
					// Unexpected error, fail the test
					assert.NoError(t, err)
					return
				}
			}

			exTd := constructLogs()
			tt.want(exTd)
			assert.Equal(t, exTd, finalLogs)
		})
	}
}

func Test_ProcessLogs_InferredResourceContext(t *testing.T) {
	tests := []struct {
		condition          string
		filteredEverything bool
		want               func(td plog.Logs)
	}{
		{
			condition:          `resource.attributes["host.name"] == "localhost"`,
			filteredEverything: true,
			want: func(_ plog.Logs) {
				// Everything should be filtered out
			},
		},
		{
			condition:          `resource.attributes["host.name"] == "wrong"`,
			filteredEverything: false,
			want: func(_ plog.Logs) {
				// Nothing should be filtered, original data remains
			},
		},
		{
			condition:          `resource.schema_url == "test_schema_url"`,
			filteredEverything: true,
			want: func(_ plog.Logs) {
				// Everything should be filtered out since schema_url matches
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			td := constructLogs()

			collection, err := common.NewLogParserCollection(componenttest.NewNopTelemetrySettings(), common.WithLogParser(filterottl.StandardLogFuncs()))
			assert.NoError(t, err)

			consumer, err := collection.ParseContextConditions(common.ContextConditions{Conditions: []string{tt.condition}})
			assert.NoError(t, err)

			err = consumer.ConsumeLogs(t.Context(), td)

			if tt.filteredEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructLogs()
				tt.want(exTd)
				assert.Equal(t, exTd, td)
			}
		})
	}
}

func Test_ProcessLogs_InferredScopeContext(t *testing.T) {
	tests := []struct {
		condition          string
		filteredEverything bool
		want               func(td plog.Logs)
	}{
		{
			condition:          `scope.name == "scope"`,
			filteredEverything: true,
			want: func(_ plog.Logs) {
				// Everything should be filtered out since scope name matches
			},
		},
		{
			condition:          `scope.version == "2"`,
			filteredEverything: false,
			want: func(_ plog.Logs) {
				// Nothing should be filtered, original data remains
			},
		},
		{
			condition:          `scope.schema_url == "test_schema_url"`,
			filteredEverything: true,
			want: func(_ plog.Logs) {
				// Everything should be filtered out since schema_url matches
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			td := constructLogs()

			collection, err := common.NewLogParserCollection(componenttest.NewNopTelemetrySettings(), common.WithLogParser(filterottl.StandardLogFuncs()))
			assert.NoError(t, err)

			consumer, err := collection.ParseContextConditions(common.ContextConditions{Conditions: []string{tt.condition}})
			assert.NoError(t, err)

			err = consumer.ConsumeLogs(t.Context(), td)

			if tt.filteredEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructLogs()
				tt.want(exTd)
				assert.Equal(t, exTd, td)
			}
		})
	}
}

func constructLogs() plog.Logs {
	td := plog.NewLogs()
	rs0 := td.ResourceLogs().AppendEmpty()
	rs0.SetSchemaUrl("test_schema_url")
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeLogs().AppendEmpty()
	rs0ils0.SetSchemaUrl("test_schema_url")
	rs0ils0.Scope().SetName("scope")
	fillLogOne(rs0ils0.LogRecords().AppendEmpty())
	fillLogTwo(rs0ils0.LogRecords().AppendEmpty())
	return td
}

func fillLogOne(log plog.LogRecord) {
	log.Body().SetStr("operationA")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	log.SetSeverityNumber(1)
	log.SetTraceID(traceID)
	log.SetSpanID(spanID)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "A|B|C")
	log.Attributes().PutStr("total.string", "123456789")
}

func fillLogTwo(log plog.LogRecord) {
	log.Body().SetStr("operationB")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "C|D")
	log.Attributes().PutStr("total.string", "345678")
}
