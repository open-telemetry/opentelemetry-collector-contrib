// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"context"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
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
	resourceAttributes map[string]interface{}
	recordAttributes   map[string]interface{}
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
			resourceAttributes: map[string]interface{}{
				"attr1": "attr1/val1",
				"attr2": "attr2/val2",
				"attr3": "attr3/val3",
			},
		},
	}

	inLogForTwoResource = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]interface{}{
				"attr1": "attr1/val1",
			},
		},
		{
			logNames: []string{"log3", "log4"},
			resourceAttributes: map[string]interface{}{
				"attr1": "attr1/val2",
			},
		},
	}

	inLogForTwoResourceWithRecordAttributes = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]interface{}{
				"attr1": "attr1/val1",
			},
			recordAttributes: map[string]interface{}{
				"rec": "rec/val1",
			},
		},
		{
			logNames: []string{"log3", "log4"},
			resourceAttributes: map[string]interface{}{
				"attr1": "attr1/val2",
			},
			recordAttributes: map[string]interface{}{
				"rec": "rec/val2",
			},
		},
	}
	inLogForThreeResourceWithRecordAttributes = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]interface{}{
				"attr1": "attr1/val1",
			},
			recordAttributes: map[string]interface{}{
				"rec": "rec/val1",
			},
		},
		{
			logNames: []string{"log3", "log4"},
			resourceAttributes: map[string]interface{}{
				"attr1": "attr1/val2",
			},
			recordAttributes: map[string]interface{}{
				"rec": "rec/val2",
			},
		},
		{
			logNames: []string{"log5"},
			resourceAttributes: map[string]interface{}{
				"attr1": "attr1/val5",
			},
			recordAttributes: map[string]interface{}{
				"rec": "rec/val5",
			},
		},
	}

	inLogForFourResource = []logWithResource{
		{
			logNames: []string{"log1"},
			resourceAttributes: map[string]interface{}{
				"attr": "attr/val1",
			},
		},
		{
			logNames: []string{"log2"},
			resourceAttributes: map[string]interface{}{
				"attr": "attr/val2",
			},
		},
		{
			logNames: []string{"log3"},
			resourceAttributes: map[string]interface{}{
				"attr": "attr/val3",
			},
		},
		{
			logNames: []string{"log4"},
			resourceAttributes: map[string]interface{}{
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
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "includeNilWithResourceAttributes",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs(inLogForResourceTest),
			outLN: [][]string{
				{"log1", "log2"},
			},
		},
		{
			name:   "includeAllWithMissingResourceAttributes",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val2"}}},
			inLogs: testResourceLogs(inLogForTwoResource),
			outLN: [][]string{
				{"log3", "log4"},
			},
		},
		{
			name:   "emptyFilterExclude",
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "excludeNilWithResourceAttributes",
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs(inLogForResourceTest),
			outLN: [][]string{
				{"log1", "log2"},
			},
		},
		{
			name:   "excludeAllWithMissingResourceAttributes",
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}}},
			inLogs: testResourceLogs(inLogForTwoResource),
			outLN: [][]string{
				{"log3", "log4"},
			},
		},
		{
			name:   "emptyFilterIncludeAndExclude",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "nilWithResourceAttributesIncludeAndExclude",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "allWithMissingResourceAttributesIncludeAndExclude",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val2"}}},
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}}},
			inLogs: testResourceLogs(inLogForTwoResource),
			outLN: [][]string{
				{"log3", "log4"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude",
			inc:    &LogMatchProperties{LogMatchType: Regexp, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val2"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log2"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude2",
			inc:    &LogMatchProperties{LogMatchType: Regexp, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val(2|3)"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log2"},
				{"log3"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude3",
			inc:    &LogMatchProperties{LogMatchType: Regexp, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val[234]"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log2"},
				{"log3"},
				{"log4"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude4",
			inc:    &LogMatchProperties{LogMatchType: Regexp, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val.*"}}},
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
			exc:    &LogMatchProperties{LogMatchType: Regexp, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val[23]"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log1"},
				{"log4"},
			},
		},
		{
			name: "matchRecordAttributeWithRegexp1",
			inc: &LogMatchProperties{
				LogMatchType: Regexp,
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
				LogMatchType: Regexp,
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
				LogMatchType: Regexp,
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
				LogMatchType: Regexp,
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
				LogMatchType:  Strict,
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
				LogMatchType:  Regexp,
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
				LogMatchType:  Strict,
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
				LogMatchType:  Regexp,
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
				LogMatchType: Strict,
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
				LogMatchType: Regexp,
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
				LogMatchType: Strict,
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
				LogMatchType: Regexp,
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
				LogMatchType: Regexp,
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
				LogMatchType: Regexp,
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
				LogMatchType: Regexp,
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
				LogMatchType: Regexp,
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
				LogMatchType: Regexp,
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
				LogMatchType: Regexp,
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
			flp, err := factory.CreateLogsProcessor(
				context.Background(),
				processortest.NewNopCreateSettings(),
				cfg,
				next,
			)
			assert.NotNil(t, flp)
			assert.Nil(t, err)

			caps := flp.Capabilities()
			assert.True(t, caps.MutatesData)
			ctx := context.Background()
			assert.NoError(t, flp.Start(ctx, nil))

			cErr := flp.ConsumeLogs(context.Background(), test.inLogs)
			assert.Nil(t, cErr)
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
	ctx := context.Background()
	proc, _ := factory.CreateLogsProcessor(
		ctx,
		processortest.NewNopCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	require.NotPanics(t, func() {
		_ = proc.ConsumeLogs(ctx, logs)
	})
}

var (
	TestLogTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestLogTimestamp = pcommon.NewTimestampFromTime(TestLogTime)

	TestObservedTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestObservedTimestamp = pcommon.NewTimestampFromTime(TestObservedTime)

	logTraceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	logSpanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
)

func TestFilterLogProcessorWithOTTL(t *testing.T) {
	tests := []struct {
		name             string
		conditions       []string
		filterEverything bool
		want             func(ld plog.Logs)
		errorMode        ottl.ErrorMode
	}{
		{
			name: "drop logs",
			conditions: []string{
				`body == "operationA"`,
			},
			want: func(ld plog.Logs) {
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
					return log.Body().AsString() == "operationA"
				})
				ld.ResourceLogs().At(0).ScopeLogs().At(1).LogRecords().RemoveIf(func(log plog.LogRecord) bool {
					return log.Body().AsString() == "operationA"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop everything by dropping all logs",
			conditions: []string{
				`IsMatch(body, "operation.*")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`IsMatch(body, "wrong name")`,
				`IsMatch(body, "operation.*")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "with error conditions",
			conditions: []string{
				`Substring("", 0, 100) == "test"`,
			},
			want:      func(ld plog.Logs) {},
			errorMode: ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := newFilterLogsProcessor(componenttest.NewNopTelemetrySettings(), &Config{Logs: LogFilters{LogConditions: tt.conditions}})
			assert.NoError(t, err)

			got, err := processor.processLogs(context.Background(), constructLogs())

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

func constructLogs() plog.Logs {
	td := plog.NewLogs()
	rs0 := td.ResourceLogs().AppendEmpty()
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeLogs().AppendEmpty()
	rs0ils0.Scope().SetName("scope1")
	fillLogOne(rs0ils0.LogRecords().AppendEmpty())
	fillLogTwo(rs0ils0.LogRecords().AppendEmpty())
	rs0ils1 := rs0.ScopeLogs().AppendEmpty()
	rs0ils1.Scope().SetName("scope2")
	fillLogOne(rs0ils1.LogRecords().AppendEmpty())
	fillLogTwo(rs0ils1.LogRecords().AppendEmpty())
	return td
}

func fillLogOne(log plog.LogRecord) {
	log.Body().SetStr("operationA")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	log.SetSeverityNumber(1)
	log.SetTraceID(logTraceID)
	log.SetSpanID(logSpanID)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "A|B|C")

}

func fillLogTwo(log plog.LogRecord) {
	log.Body().SetStr("operationB")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "C|D")

}
