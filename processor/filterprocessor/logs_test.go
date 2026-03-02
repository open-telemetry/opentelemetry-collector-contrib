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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/condition"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadatatest"
)

type logNameTest struct {
	name   string
	inc    *LogMatchProperties
	exc    *LogMatchProperties
	inLogs plog.Logs
	outLN  [][]string // output Log names per Resource
}

type logWithResource struct {
	logNames           []string
	resourceAttributes map[string]any
	recordAttributes   map[string]any
	severityText       string
	body               string
	severityNumber     plog.SeverityNumber
}

var (
	inLogNames = []string{
		"full_name_match",
		"random",
	}

	inLogForResourceTest = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val1",
				"attr2": "attr2/val2",
				"attr3": "attr3/val3",
			},
		},
	}

	inLogForTwoResource = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val1",
			},
		},
		{
			logNames: []string{"log3", "log4"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val2",
			},
		},
	}

	inLogForTwoResourceWithRecordAttributes = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val1",
			},
			recordAttributes: map[string]any{
				"rec": "rec/val1",
			},
		},
		{
			logNames: []string{"log3", "log4"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val2",
			},
			recordAttributes: map[string]any{
				"rec": "rec/val2",
			},
		},
	}
	inLogForThreeResourceWithRecordAttributes = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val1",
			},
			recordAttributes: map[string]any{
				"rec": "rec/val1",
			},
		},
		{
			logNames: []string{"log3", "log4"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val2",
			},
			recordAttributes: map[string]any{
				"rec": "rec/val2",
			},
		},
		{
			logNames: []string{"log5"},
			resourceAttributes: map[string]any{
				"attr1": "attr1/val5",
			},
			recordAttributes: map[string]any{
				"rec": "rec/val5",
			},
		},
	}

	inLogForFourResource = []logWithResource{
		{
			logNames: []string{"log1"},
			resourceAttributes: map[string]any{
				"attr": "attr/val1",
			},
		},
		{
			logNames: []string{"log2"},
			resourceAttributes: map[string]any{
				"attr": "attr/val2",
			},
		},
		{
			logNames: []string{"log3"},
			resourceAttributes: map[string]any{
				"attr": "attr/val3",
			},
		},
		{
			logNames: []string{"log4"},
			resourceAttributes: map[string]any{
				"attr": "attr/val4",
			},
		},
	}

	inLogForSeverityText = []logWithResource{
		{
			logNames:     []string{"log1"},
			severityText: "DEBUG",
		},
		{
			logNames:     []string{"log2"},
			severityText: "DEBUG2",
		},
		{
			logNames:     []string{"log3"},
			severityText: "INFO",
		},
		{
			logNames:     []string{"log4"},
			severityText: "WARN",
		},
	}

	inLogForBody = []logWithResource{
		{
			logNames: []string{"log1"},
			body:     "This is a log body",
		},
		{
			logNames: []string{"log2"},
			body:     "This is also a log body",
		},
		{
			logNames: []string{"log3"},
			body:     "test1",
		},
		{
			logNames: []string{"log4"},
			body:     "test2",
		},
	}

	inLogForSeverityNumber = []logWithResource{
		{
			logNames:       []string{"log1"},
			severityNumber: plog.SeverityNumberDebug,
		},
		{
			logNames:       []string{"log2"},
			severityNumber: plog.SeverityNumberInfo,
		},
		{
			logNames:       []string{"log3"},
			severityNumber: plog.SeverityNumberError,
		},
		{
			logNames:       []string{"log4"},
			severityNumber: plog.SeverityNumberUnspecified,
		},
	}

	standardLogTests = []logNameTest{
		{
			name:   "emptyFilterInclude",
			inc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "includeNilWithResourceAttributes",
			inc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs(inLogForResourceTest),
			outLN: [][]string{
				{"log1", "log2"},
			},
		},
		{
			name:   "includeAllWithMissingResourceAttributes",
			inc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val2"}}},
			inLogs: testResourceLogs(inLogForTwoResource),
			outLN: [][]string{
				{"log3", "log4"},
			},
		},
		{
			name:   "emptyFilterExclude",
			exc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "excludeNilWithResourceAttributes",
			exc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs(inLogForResourceTest),
			outLN: [][]string{
				{"log1", "log2"},
			},
		},
		{
			name:   "excludeAllWithMissingResourceAttributes",
			exc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}}},
			inLogs: testResourceLogs(inLogForTwoResource),
			outLN: [][]string{
				{"log3", "log4"},
			},
		},
		{
			name:   "emptyFilterIncludeAndExclude",
			inc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{}},
			exc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "nilWithResourceAttributesIncludeAndExclude",
			inc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{}},
			exc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "allWithMissingResourceAttributesIncludeAndExclude",
			inc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val2"}}},
			exc:    &LogMatchProperties{LogMatchType: strictType, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}}},
			inLogs: testResourceLogs(inLogForTwoResource),
			outLN: [][]string{
				{"log3", "log4"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude",
			inc:    &LogMatchProperties{LogMatchType: regexpType, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val2"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log2"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude2",
			inc:    &LogMatchProperties{LogMatchType: regexpType, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val(2|3)"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log2"},
				{"log3"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude3",
			inc:    &LogMatchProperties{LogMatchType: regexpType, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val[234]"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log2"},
				{"log3"},
				{"log4"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude4",
			inc:    &LogMatchProperties{LogMatchType: regexpType, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val.*"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log1"},
				{"log2"},
				{"log3"},
				{"log4"},
			},
		},
		{
			name:   "matchAttributesWithRegexpExclude",
			exc:    &LogMatchProperties{LogMatchType: regexpType, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val[23]"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log1"},
				{"log4"},
			},
		},
		{
			name: "matchRecordAttributeWithRegexp1",
			inc: &LogMatchProperties{
				LogMatchType: regexpType,
				RecordAttributes: []filterconfig.Attribute{
					{
						Key:   "rec",
						Value: "rec/val[1]",
					},
				},
			},
			inLogs: testResourceLogs(inLogForTwoResourceWithRecordAttributes),
			outLN: [][]string{
				{"log1", "log2"},
			},
		},
		{
			name: "matchRecordAttributeWithRegexp2",
			inc: &LogMatchProperties{
				LogMatchType: regexpType,
				RecordAttributes: []filterconfig.Attribute{
					{
						Key:   "rec",
						Value: "rec/val[^2]",
					},
				},
			},
			inLogs: testResourceLogs(inLogForTwoResourceWithRecordAttributes),
			outLN: [][]string{
				{"log1", "log2"},
			},
		},
		{
			name: "matchRecordAttributeWithRegexp2",
			inc: &LogMatchProperties{
				LogMatchType: regexpType,
				RecordAttributes: []filterconfig.Attribute{
					{
						Key:   "rec",
						Value: "rec/val[1|2]",
					},
				},
			},
			inLogs: testResourceLogs(inLogForTwoResourceWithRecordAttributes),
			outLN: [][]string{
				{"log1", "log2"},
				{"log3", "log4"},
			},
		},
		{
			name: "matchRecordAttributeWithRegexp3",
			inc: &LogMatchProperties{
				LogMatchType: regexpType,
				RecordAttributes: []filterconfig.Attribute{
					{
						Key:   "rec",
						Value: "rec/val[1|5]",
					},
				},
			},
			inLogs: testResourceLogs(inLogForThreeResourceWithRecordAttributes),
			outLN: [][]string{
				{"log1", "log2"},
				{"log5"},
			},
		},
		{
			name: "includeRecordSeverityStrict",
			inc: &LogMatchProperties{
				LogMatchType:  strictType,
				SeverityTexts: []string{"INFO", "DEBUG2"},
			},
			inLogs: testResourceLogs(inLogForSeverityText),
			outLN: [][]string{
				{"log2"},
				{"log3"},
			},
		},
		{
			name: "includeRecordSeverityRegexp",
			inc: &LogMatchProperties{
				LogMatchType:  regexpType,
				SeverityTexts: []string{"DEBUG[1-4]?"},
			},
			inLogs: testResourceLogs(inLogForSeverityText),
			outLN: [][]string{
				{"log1"},
				{"log2"},
			},
		},
		{
			name: "excludeRecordSeverityStrict",
			exc: &LogMatchProperties{
				LogMatchType:  strictType,
				SeverityTexts: []string{"INFO", "DEBUG"},
			},
			inLogs: testResourceLogs(inLogForSeverityText),
			outLN: [][]string{
				{"log2"},
				{"log4"},
			},
		},
		{
			name: "excludeRecordSeverityRegexp",
			exc: &LogMatchProperties{
				LogMatchType:  regexpType,
				SeverityTexts: []string{"^[DI]"},
			},
			inLogs: testResourceLogs(inLogForSeverityText),
			outLN: [][]string{
				{"log4"},
			},
		},
		{
			name: "includeRecordBodyStrict",
			inc: &LogMatchProperties{
				LogMatchType: strictType,
				LogBodies:    []string{"test1", "test2", "no match"},
			},
			inLogs: testResourceLogs(inLogForBody),
			outLN: [][]string{
				{"log3"},
				{"log4"},
			},
		},
		{
			name: "includeRecordBodyRegexp",
			inc: &LogMatchProperties{
				LogMatchType: regexpType,
				LogBodies:    []string{"^This"},
			},
			inLogs: testResourceLogs(inLogForBody),
			outLN: [][]string{
				{"log1"},
				{"log2"},
			},
		},
		{
			name: "excludeRecordBodyStrict",
			exc: &LogMatchProperties{
				LogMatchType: strictType,
				LogBodies:    []string{"test1", "test2", "no match"},
			},
			inLogs: testResourceLogs(inLogForBody),
			outLN: [][]string{
				{"log1"},
				{"log2"},
			},
		},
		{
			name: "excludeRecordBodyRegexp",
			exc: &LogMatchProperties{
				LogMatchType: regexpType,
				LogBodies:    []string{"^This"},
			},
			inLogs: testResourceLogs(inLogForBody),
			outLN: [][]string{
				{"log3"},
				{"log4"},
			},
		},
		{
			name: "includeMinSeverityINFO",
			inc: &LogMatchProperties{
				LogMatchType: regexpType,
				SeverityNumberProperties: &LogSeverityNumberMatchProperties{
					Min: logSeverity("INFO"),
				},
			},
			inLogs: testResourceLogs(inLogForSeverityNumber),
			outLN: [][]string{
				{"log2"},
				{"log3"},
			},
		},
		{
			name: "includeMinSeverityDEBUG",
			inc: &LogMatchProperties{
				LogMatchType: regexpType,
				SeverityNumberProperties: &LogSeverityNumberMatchProperties{
					Min: logSeverity("DEBUG"),
				},
			},
			inLogs: testResourceLogs(inLogForSeverityNumber),
			outLN: [][]string{
				{"log1"},
				{"log2"},
				{"log3"},
			},
		},
		{
			name: "includeMinSeverityFATAL+undefined",
			inc: &LogMatchProperties{
				LogMatchType: regexpType,
				SeverityNumberProperties: &LogSeverityNumberMatchProperties{
					Min:            logSeverity("FATAL"),
					MatchUndefined: true,
				},
			},
			inLogs: testResourceLogs(inLogForSeverityNumber),
			outLN: [][]string{
				{"log4"},
			},
		},
		{
			name: "excludeMinSeverityINFO",
			exc: &LogMatchProperties{
				LogMatchType: regexpType,
				SeverityNumberProperties: &LogSeverityNumberMatchProperties{
					Min: logSeverity("INFO"),
				},
			},
			inLogs: testResourceLogs(inLogForSeverityNumber),
			outLN: [][]string{
				{"log1"},
				{"log4"},
			},
		},
		{
			name: "excludeMinSeverityTRACE",
			exc: &LogMatchProperties{
				LogMatchType: regexpType,
				SeverityNumberProperties: &LogSeverityNumberMatchProperties{
					Min: logSeverity("TRACE"),
				},
			},
			inLogs: testResourceLogs(inLogForSeverityNumber),
			outLN: [][]string{
				{"log4"},
			},
		},
		{
			name: "excludeMinSeverityINFO+undefined",
			exc: &LogMatchProperties{
				LogMatchType: regexpType,
				SeverityNumberProperties: &LogSeverityNumberMatchProperties{
					Min:            logSeverity("INFO"),
					MatchUndefined: true,
				},
			},
			inLogs: testResourceLogs(inLogForSeverityNumber),
			outLN: [][]string{
				{"log1"},
			},
		},
	}
)

func TestFilterLogProcessor(t *testing.T) {
	for _, test := range standardLogTests {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the filter log processor
			next := new(consumertest.LogsSink)
			cfg := &Config{
				Logs: LogFilters{
					Include: test.inc,
					Exclude: test.exc,
				},
			}
			factory := NewFactory()
			flp, err := factory.CreateLogs(
				t.Context(),
				processortest.NewNopSettings(metadata.Type),
				cfg,
				next,
			)
			assert.NotNil(t, flp)
			assert.NoError(t, err)

			caps := flp.Capabilities()
			assert.True(t, caps.MutatesData)
			ctx := t.Context()
			assert.NoError(t, flp.Start(ctx, componenttest.NewNopHost()))

			cErr := flp.ConsumeLogs(t.Context(), test.inLogs)
			assert.NoError(t, cErr)
			got := next.AllLogs()

			require.Len(t, got, 1)
			rLogs := got[0].ResourceLogs()
			assert.Equal(t, len(test.outLN), rLogs.Len())

			for i, wantOut := range test.outLN {
				gotLogs := rLogs.At(i).ScopeLogs().At(0).LogRecords()
				assert.Equal(t, len(wantOut), gotLogs.Len())
				for idx := range wantOut {
					val, ok := gotLogs.At(idx).Attributes().Get("name")
					require.True(t, ok)
					assert.Equal(t, wantOut[idx], val.AsString())
				}
			}
			assert.NoError(t, flp.Shutdown(ctx))
		})
	}
}

func testResourceLogs(lwrs []logWithResource) plog.Logs {
	ld := plog.NewLogs()

	for i, lwr := range lwrs {
		rl := ld.ResourceLogs().AppendEmpty()

		// Add resource level attributes
		//nolint:errcheck
		rl.Resource().Attributes().FromRaw(lwr.resourceAttributes)
		ls := rl.ScopeLogs().AppendEmpty().LogRecords()
		for _, name := range lwr.logNames {
			l := ls.AppendEmpty()
			// Add record level attributes
			//nolint:errcheck
			l.Attributes().FromRaw(lwrs[i].recordAttributes)
			l.Attributes().PutStr("name", name)
			// Set body & severity fields
			l.Body().SetStr(lwr.body)
			l.SetSeverityText(lwr.severityText)
			l.SetSeverityNumber(lwr.severityNumber)
		}
	}
	return ld
}

func TestNilResourceLogs(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs()
	rls.AppendEmpty()
	requireNotPanicsLogs(t, logs)
}

func TestNilILL(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs()
	rl := rls.AppendEmpty()
	ills := rl.ScopeLogs()
	ills.AppendEmpty()
	requireNotPanicsLogs(t, logs)
}

func TestNilLog(t *testing.T) {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs()
	rl := rls.AppendEmpty()
	ills := rl.ScopeLogs()
	sl := ills.AppendEmpty()
	ls := sl.LogRecords()
	ls.AppendEmpty()
	requireNotPanicsLogs(t, logs)
}

func requireNotPanicsLogs(t *testing.T, logs plog.Logs) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	pcfg := cfg.(*Config)
	pcfg.Logs = LogFilters{
		Exclude: nil,
	}
	ctx := t.Context()
	proc, _ := factory.CreateLogs(
		ctx,
		processortest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	require.NotPanics(t, func() {
		_ = proc.ConsumeLogs(ctx, logs)
	})
}

var (
	testLogTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	testLogTimestamp = pcommon.NewTimestampFromTime(testLogTime)

	testObservedTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	testObservedTimestamp = pcommon.NewTimestampFromTime(testObservedTime)

	logTraceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	logSpanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
)

func TestFilterLogProcessorWithOTTL(t *testing.T) {
	tests := []struct {
		name             string
		conditions       LogFilters
		filterEverything bool
		want             func(ld plog.Logs)
	}{
		{
			name: "drop resource",
			conditions: LogFilters{
				ResourceConditions: []string{
					`attributes["host.name"] == "localhost"`,
				},
			},
			filterEverything: true,
		},
		{
			name: "drop logs",
			conditions: LogFilters{
				LogConditions: []string{
					`body == "operationA"`,
				},
			},
			want: func(ld plog.Logs) {
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
					return log.Body().AsString() == "operationA"
				})
				ld.ResourceLogs().At(0).ScopeLogs().At(1).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
					return log.Body().AsString() == "operationA"
				})
			},
		},
		{
			name: "drop everything by dropping all logs",
			conditions: LogFilters{
				LogConditions: []string{
					`IsMatch(body, "operation.*")`,
				},
			},
			filterEverything: true,
		},
		{
			name: "multiple conditions",
			conditions: LogFilters{
				LogConditions: []string{
					`IsMatch(body, "wrong name")`,
					`IsMatch(body, "operation.*")`,
				},
			},
			filterEverything: true,
		},
		{
			name: "with error conditions",
			conditions: LogFilters{
				LogConditions: []string{
					`Substring("", 0, 100) == "test"`,
				},
			},
			want: func(_ plog.Logs) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Logs: tt.conditions, logFunctions: defaultLogFunctionsMap()}
			processor, err := newFilterLogsProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processLogs(t.Context(), constructLogs())

			if tt.filterEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				exTd := constructLogs()
				tt.want(exTd)
				assert.Equal(t, exTd, got)
			}
		})
	}
}

func Test_ProcessLogs_DefinedContext(t *testing.T) {
	tests := []struct {
		name              string
		contextConditions []condition.ContextConditions
		filterEverything  bool
		want              func(ld plog.Logs)
		errorMode         ottl.ErrorMode
	}{
		{
			name: "resource: drop everything",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`schema_url == "test_schema_url"`}, Context: "resource"},
			},
			filterEverything: true,
		},
		{
			name: "resource: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`attributes["host.name"] == "localhost"`}, Context: "resource"},
			},
			filterEverything: true,
		},
		{
			name: "scope: drop everything",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`schema_url == "test_schema_url"`}, Context: "scope"},
			},
			filterEverything: true,
		},
		{
			name: "scope: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`attributes["lib"] == "awesomelib"`}, Context: "scope"},
			},
			want: func(_ plog.Logs) {},
		},
		{
			name: "scope: drop by name",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`name == "scope0"`}, Context: "scope"},
			},
			want: func(ld plog.Logs) {
				ld.ResourceLogs().At(0).ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
					return sl.Scope().Name() == "scope0"
				})
			},
		},
		{
			name: "log: drop by attributes",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`attributes["total.string"] == "123456789"`}, Context: "log"},
			},
			want: func(ld plog.Logs) {
				rl := ld.ResourceLogs().At(0)
				for i := 0; i < rl.ScopeLogs().Len(); i++ {
					rl.ScopeLogs().At(i).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
						v, ok := log.Attributes().Get("total.string")
						if ok {
							return v.AsString() == "123456789"
						}
						return false
					})
				}
			},
		},
		{
			name: "log: drop by function",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`IsMatch(body, "operationA")`}, Context: "log"},
			},
			want: func(ld plog.Logs) {
				rl := ld.ResourceLogs().At(0)
				for i := 0; i < rl.ScopeLogs().Len(); i++ {
					rl.ScopeLogs().At(i).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
						return log.Body().AsString() == "operationA"
					})
				}
			},
		},
		{
			name: "log: drop by enum",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`severity_number < SEVERITY_NUMBER_WARN`}, Context: "log"},
			},
			want: func(ld plog.Logs) {
				rl := ld.ResourceLogs().At(0)
				for i := 0; i < rl.ScopeLogs().Len(); i++ {
					rl.ScopeLogs().At(i).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
						return log.SeverityNumber() < plog.SeverityNumberWarn
					})
				}
			},
		},
		{
			name: "mixed conditions",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`body == "operationA"`}, Context: "log"},
				{Conditions: []string{`name == "scope1"`}, Context: "scope"},
			},
			want: func(ld plog.Logs) {
				rl := ld.ResourceLogs().At(0)
				for i := 0; i < rl.ScopeLogs().Len(); i++ {
					rl.ScopeLogs().At(i).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
						return log.Body().AsString() == "operationA"
					})
					rl.ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
						return sl.Scope().Name() == "scope1"
					})
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.LogConditions = tt.contextConditions
			processor, err := newFilterLogsProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processLogs(t.Context(), constructLogs())

			if tt.filterEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructLogs()
				tt.want(exTd)
				assert.Equal(t, exTd, got)
			}
		})
	}
}

func Test_ProcessLogs_InferredContext(t *testing.T) {
	tests := []struct {
		name              string
		contextConditions []condition.ContextConditions
		filterEverything  bool
		want              func(ld plog.Logs)
		errorMode         ottl.ErrorMode
		input             func() plog.Logs
	}{
		{
			name: "resource: drop everything",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`resource.schema_url == "test_schema_url"`}},
			},
			filterEverything: true,
			input:            constructLogs,
		},
		{
			name: "resource: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`resource.attributes["host.name"] == "localhost"`}},
			},
			filterEverything: true,
			input:            constructLogs,
		},
		{
			name: "scope: drop everything",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`scope.schema_url == "test_schema_url"`}},
			},
			filterEverything: true,
			input:            constructLogs,
		},
		{
			name: "scope: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`scope.attributes["lib"] == "awesomelib"`}},
			},
			want:  func(_ plog.Logs) {},
			input: constructLogs,
		},
		{
			name: "scope: drop by name",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`scope.name == "scope0"`}},
			},
			want: func(ld plog.Logs) {
				ld.ResourceLogs().At(0).ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
					return sl.Scope().Name() == "scope0"
				})
			},
			input: constructLogs,
		},
		{
			name: "log: drop by attributes",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`log.attributes["total.string"] == "123456789"`}},
			},
			want: func(ld plog.Logs) {
				rl := ld.ResourceLogs().At(0)
				for i := 0; i < rl.ScopeLogs().Len(); i++ {
					rl.ScopeLogs().At(i).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
						v, ok := log.Attributes().Get("total.string")
						if ok {
							return v.AsString() == "123456789"
						}
						return false
					})
				}
			},
			input: constructLogs,
		},
		{
			name: "log: drop by function",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`IsMatch(log.body, "operationA")`}},
			},
			want: func(ld plog.Logs) {
				rl := ld.ResourceLogs().At(0)
				for i := 0; i < rl.ScopeLogs().Len(); i++ {
					rl.ScopeLogs().At(i).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
						return log.Body().AsString() == "operationA"
					})
				}
			},
			input: constructLogs,
		},
		{
			name: "log: drop by enum",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`log.severity_number < SEVERITY_NUMBER_WARN`}},
			},
			want: func(ld plog.Logs) {
				rl := ld.ResourceLogs().At(0)
				for i := 0; i < rl.ScopeLogs().Len(); i++ {
					rl.ScopeLogs().At(i).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
						return log.SeverityNumber() < plog.SeverityNumberWarn
					})
				}
			},
			input: constructLogs,
		},
		{
			name: "inferring mixed contexts",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{
					`log.body == "operationA"`,
					`scope.name == "scope1"`,
				}},
			},
			want: func(ld plog.Logs) {
				rl := ld.ResourceLogs().At(0)
				for i := 0; i < rl.ScopeLogs().Len(); i++ {
					rl.ScopeLogs().At(i).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
						return log.Body().AsString() == "operationA"
					})
					rl.ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
						return sl.Scope().Name() == "scope1"
					})
				}
			},
			input: constructLogs,
		},
		{
			name: "zero-record lower-context: resource and log with no log records",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{
					`resource.attributes["host.name"] == "localhost"`,
					`log.body == "test"`,
				}},
			},
			filterEverything: true,
			input:            constructLogsWithEmptyLogRecords,
		},
		{
			name: "group by context: conditions in same group are respected",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{
					`resource.attributes["host.name"] == "host1"`,
					`resource.attributes["host.name"] == "host3"`,
					`scope.name == "scope3"`,
					`scope.name == "scope4"`,
				}},
			},
			filterEverything: true,
			input:            constructLogsMultiResources,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.LogConditions = tt.contextConditions
			processor, err := newFilterLogsProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processLogs(t.Context(), tt.input())

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

func Test_ProcessLogs_ErrorMode(t *testing.T) {
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
			name: "log",
		},
	}

	for _, errMode := range []ottl.ErrorMode{ottl.PropagateError, ottl.IgnoreError, ottl.SilentError} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s:%s", tt.name, errMode), func(t *testing.T) {
				cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
				cfg.LogConditions = []condition.ContextConditions{
					{Conditions: []string{`ParseJSON("1")`}, Context: condition.ContextID(tt.name)},
				}
				cfg.ErrorMode = errMode
				processor, err := newFilterLogsProcessor(processortest.NewNopSettings(metadata.Type), cfg)
				assert.NoError(t, err)

				_, err = processor.processLogs(t.Context(), constructLogs())

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

func Test_ProcessLogs_ConditionsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		conditions    []condition.ContextConditions
		want          func(td plog.Logs)
		wantErrorWith string
	}{
		{
			name:      "resource: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("resource")},
				{Conditions: []string{`not IsMatch(resource.attributes["host.name"], ".*")`}, Context: condition.ContextID("resource")},
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
			want: func(_ plog.Logs) {},
		},
		{
			name:      "scope: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("scope")},
				{Conditions: []string{`scope.schema_url != "test_schema_url"`}, Context: condition.ContextID("scope")},
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
			conditions: []condition.ContextConditions{
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("scope")},
				{Conditions: []string{`scope.attributes["pass"] == ParseJSON("true")`}, Context: condition.ContextID("scope")},
			},
			wantErrorWith: "could not convert parsed value of type bool to JSON object",
		},
		{
			name:      "log: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`log.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("log")},
				{Conditions: []string{`not IsMatch(log.body, ".*")`}, Context: condition.ContextID("log")},
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
			conditions: []condition.ContextConditions{
				{Conditions: []string{`log.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("log")},
				{Conditions: []string{`log.attributes["pass"] == ParseJSON("true")`}, Context: condition.ContextID("log")},
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
			want: func(ld plog.Logs) {
				ld.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
					v, _ := rl.Resource().Attributes().Get("host.name")
					return v.AsString() == ""
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.LogConditions = tt.conditions
			cfg.ErrorMode = tt.errorMode

			processor, err := newFilterLogsProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processLogs(t.Context(), constructLogs())
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}

			assert.NoError(t, err)
			exTd := constructLogs()
			tt.want(exTd)
			assert.Equal(t, exTd, got)
		})
	}
}

func Test_Logs_NonDefaultFunctions(t *testing.T) {
	type testCase struct {
		name          string
		conditions    []condition.ContextConditions
		wantErrorWith string
		logFunctions  map[string]ottl.Factory[*ottllog.TransformContext]
	}

	tests := []testCase{
		{
			name: "log funcs : statement with added log func",
			conditions: []condition.ContextConditions{
				{
					Context:    condition.ContextID("log"),
					Conditions: []string{`IsMatch(body, TestLogFunc())`},
				},
			},
			logFunctions: map[string]ottl.Factory[*ottllog.TransformContext]{
				"IsMatch":     defaultLogFunctionsMap()["IsMatch"],
				"TestLogFunc": NewLogFuncFactory[*ottllog.TransformContext](),
			},
		},
		{
			name: "log funcs : statement with missing log func",
			conditions: []condition.ContextConditions{
				{
					Context:    condition.ContextID("log"),
					Conditions: []string{`IsMatch(body, TestLogFunc())`},
				},
			},
			wantErrorWith: `undefined function "TestLogFunc"`,
			logFunctions:  defaultLogFunctionsMap(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.LogConditions = tt.conditions
			cfg.logFunctions = tt.logFunctions

			_, err := newFilterLogsProcessor(processortest.NewNopSettings(metadata.Type), cfg)

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

type LogFuncArguments[K any] struct{}

func createLogFunc[K any](_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(_ context.Context, _ K) (any, error) {
		return nil, nil
	}, nil
}

func NewLogFuncFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TestLogFunc", &LogFuncArguments[K]{}, createLogFunc[K])
}

func TestFilterLogProcessorTelemetry(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting
	processor, err := newFilterLogsProcessor(metadatatest.NewSettings(tel), &Config{
		Logs:         LogFilters{LogConditions: []string{`IsMatch(body, "operationA")`}},
		logFunctions: defaultLogFunctionsMap(),
	})
	assert.NoError(t, err)

	_, err = processor.processLogs(t.Context(), constructLogs())
	assert.NoError(t, err)

	metadatatest.AssertEqualProcessorFilterLogsFiltered(t, tel, []metricdata.DataPoint[int64]{
		{
			Value:      2,
			Attributes: attribute.NewSet(attribute.String("filter", "filter")),
		},
	}, metricdatatest.IgnoreTimestamp())
}

func constructLogsWithEmptyLogRecords() plog.Logs {
	td := plog.NewLogs()
	rs := td.ResourceLogs().AppendEmpty()
	rs.Resource().Attributes().PutStr("host.name", "localhost")
	sl := rs.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("scope0")
	return td
}

func constructLogsMultiResources() plog.Logs {
	td := plog.NewLogs()

	rs0 := td.ResourceLogs().AppendEmpty()
	rs0.Resource().Attributes().PutStr("host.name", "host1")
	rs0s0 := rs0.ScopeLogs().AppendEmpty()
	rs0s0.Scope().SetName("scope1")
	fillLogOne(rs0s0.LogRecords().AppendEmpty())
	fillLogOne(rs0s0.LogRecords().AppendEmpty())
	rs0s1 := rs0.ScopeLogs().AppendEmpty()
	rs0s1.Scope().SetName("scope2")
	fillLogOne(rs0s1.LogRecords().AppendEmpty())
	fillLogOne(rs0s1.LogRecords().AppendEmpty())

	rs1 := td.ResourceLogs().AppendEmpty()
	rs1.Resource().Attributes().PutStr("host.name", "host2")
	rs1s0 := rs1.ScopeLogs().AppendEmpty()
	rs1s0.Scope().SetName("scope3")
	fillLogTwo(rs1s0.LogRecords().AppendEmpty())
	fillLogTwo(rs1s0.LogRecords().AppendEmpty())
	rs1s1 := rs1.ScopeLogs().AppendEmpty()
	rs1s1.Scope().SetName("scope4")
	fillLogTwo(rs1s1.LogRecords().AppendEmpty())
	fillLogTwo(rs1s1.LogRecords().AppendEmpty())

	rs2 := td.ResourceLogs().AppendEmpty()
	rs2.Resource().Attributes().PutStr("host.name", "host3")
	return td
}

func constructLogs() plog.Logs {
	td := plog.NewLogs()
	rs0 := td.ResourceLogs().AppendEmpty()
	rs0.SetSchemaUrl("test_schema_url")
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0s0 := rs0.ScopeLogs().AppendEmpty()
	rs0s0.SetSchemaUrl("test_schema_url")
	rs0s0.Scope().SetName("scope0")
	fillLogOne(rs0s0.LogRecords().AppendEmpty())
	fillLogTwo(rs0s0.LogRecords().AppendEmpty())
	rs0s1 := rs0.ScopeLogs().AppendEmpty()
	rs0s1.SetSchemaUrl("test_schema_url")
	rs0s1.Scope().SetName("scope1")
	fillLogOne(rs0s1.LogRecords().AppendEmpty())
	fillLogTwo(rs0s1.LogRecords().AppendEmpty())
	return td
}

func fillLogOne(log plog.LogRecord) {
	log.Body().SetStr("operationA")
	log.SetTimestamp(testLogTimestamp)
	log.SetObservedTimestamp(testObservedTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	log.SetSeverityNumber(1)
	log.SetTraceID(logTraceID)
	log.SetSpanID(logSpanID)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "A|B|C")
	log.Attributes().PutStr("total.string", "123456789")
	log.SetSeverityNumber(plog.SeverityNumberWarn)
}

func fillLogTwo(log plog.LogRecord) {
	log.Body().SetStr("operationB")
	log.SetTimestamp(testLogTimestamp)
	log.SetObservedTimestamp(testObservedTimestamp)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "C|D")
	log.Attributes().PutStr("total.string", "345678")
	log.SetSeverityNumber(plog.SeverityNumberInfo)
}
