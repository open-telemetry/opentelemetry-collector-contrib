// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl_test

import (
	"fmt"
	"strconv"
	"testing"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

const benchmarkContextPoolSize = 32

var (
	parserStatementsCountSink int
	conditionSequenceResult   bool
)

func BenchmarkParserParseStatements(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()

	logParser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create log parser: %v", err)
	}

	spanParser, err := ottlspan.NewParser(ottlfuncs.StandardFuncs[ottlspan.TransformContext](), settings, ottlspan.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create span parser: %v", err)
	}

	metricParser, err := ottlmetric.NewParser(ottlfuncs.StandardFuncs[ottlmetric.TransformContext](), settings, ottlmetric.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create metric parser: %v", err)
	}

	logScenarios := []struct {
		name       string
		statements []string
	}{
		{name: "small", statements: buildLogStatements(10)},
		{name: "medium", statements: buildLogStatements(50)},
		{name: "large", statements: buildLogStatements(200)},
	}

	for _, scenario := range logScenarios {
		b.Run("logs/"+scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				parsed, err := logParser.ParseStatements(scenario.statements)
				if err != nil {
					b.Fatalf("failed to parse log statements: %v", err)
				}
				parserStatementsCountSink = len(parsed)
			}
		})
	}

	spanScenarios := []struct {
		name       string
		statements []string
	}{
		{name: "small", statements: buildSpanStatements(10)},
		{name: "medium", statements: buildSpanStatements(50)},
		{name: "large", statements: buildSpanStatements(200)},
	}

	for _, scenario := range spanScenarios {
		b.Run("spans/"+scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				parsed, err := spanParser.ParseStatements(scenario.statements)
				if err != nil {
					b.Fatalf("failed to parse span statements: %v", err)
				}
				parserStatementsCountSink = len(parsed)
			}
		})
	}

	metricScenarios := []struct {
		name       string
		statements []string
	}{
		{name: "small", statements: buildMetricStatements(10)},
		{name: "medium", statements: buildMetricStatements(50)},
		{name: "large", statements: buildMetricStatements(200)},
	}

	for _, scenario := range metricScenarios {
		b.Run("metrics/"+scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				parsed, err := metricParser.ParseStatements(scenario.statements)
				if err != nil {
					b.Fatalf("failed to parse metric statements: %v", err)
				}
				parserStatementsCountSink = len(parsed)
			}
		})
	}
}

func BenchmarkStatementSequenceExecuteLogs(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create log parser: %v", err)
	}

	scenarios := []struct {
		name       string
		statements []string
	}{
		{name: "small", statements: buildLogStatements(10)},
		{name: "medium", statements: buildLogStatements(50)},
		{name: "large", statements: buildLogStatements(200)},
	}

	ctx := b.Context()

	for _, scenario := range scenarios {
		parsed, err := parser.ParseStatements(scenario.statements)
		if err != nil {
			b.Fatalf("failed to parse log statements: %v", err)
		}
		sequence := ottllog.NewStatementSequence(parsed, settings)

		contexts := make([]ottllog.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newBenchmarkLogContext(len(scenario.statements))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := sequence.Execute(ctx, contexts[i%len(contexts)]); err != nil {
					b.Fatalf("failed to execute log statements: %v", err)
				}
			}
		})
	}
}

func BenchmarkStatementSequenceExecuteSpans(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottlspan.NewParser(ottlfuncs.StandardFuncs[ottlspan.TransformContext](), settings, ottlspan.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create span parser: %v", err)
	}

	scenarios := []struct {
		name       string
		statements []string
	}{
		{name: "small", statements: buildSpanStatements(10)},
		{name: "medium", statements: buildSpanStatements(50)},
		{name: "large", statements: buildSpanStatements(200)},
	}

	ctx := b.Context()

	for _, scenario := range scenarios {
		parsed, err := parser.ParseStatements(scenario.statements)
		if err != nil {
			b.Fatalf("failed to parse span statements: %v", err)
		}
		sequence := ottlspan.NewStatementSequence(parsed, settings)

		contexts := make([]ottlspan.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newBenchmarkSpanContext(len(scenario.statements))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := sequence.Execute(ctx, contexts[i%len(contexts)]); err != nil {
					b.Fatalf("failed to execute span statements: %v", err)
				}
			}
		})
	}
}

func BenchmarkStatementSequenceExecuteMetrics(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottlmetric.NewParser(ottlfuncs.StandardFuncs[ottlmetric.TransformContext](), settings, ottlmetric.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create metric parser: %v", err)
	}

	scenarios := []struct {
		name       string
		statements []string
	}{
		{name: "small", statements: buildMetricStatements(10)},
		{name: "medium", statements: buildMetricStatements(50)},
		{name: "large", statements: buildMetricStatements(200)},
	}

	ctx := b.Context()

	for _, scenario := range scenarios {
		parsed, err := parser.ParseStatements(scenario.statements)
		if err != nil {
			b.Fatalf("failed to parse metric statements: %v", err)
		}
		sequence := ottlmetric.NewStatementSequence(parsed, settings)

		contexts := make([]ottlmetric.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newBenchmarkMetricContext(len(scenario.statements))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := sequence.Execute(ctx, contexts[i%len(contexts)]); err != nil {
					b.Fatalf("failed to execute metric statements: %v", err)
				}
			}
		})
	}
}

func BenchmarkConditionSequenceEvalLogs(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create log parser: %v", err)
	}

	conditions := []struct {
		name       string
		predicates []string
	}{
		{name: "small", predicates: buildLogConditions(10)},
		{name: "medium", predicates: buildLogConditions(50)},
		{name: "large", predicates: buildLogConditions(200)},
	}

	ctx := b.Context()

	for _, scenario := range conditions {
		parsed, err := parser.ParseConditions(scenario.predicates)
		if err != nil {
			b.Fatalf("failed to parse log conditions: %v", err)
		}
		sequence := ottllog.NewConditionSequence(parsed, settings)

		contexts := make([]ottllog.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newBenchmarkLogContext(len(scenario.predicates))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := sequence.Eval(ctx, contexts[i%len(contexts)])
				if err != nil {
					b.Fatalf("failed to evaluate log conditions: %v", err)
				}
				conditionSequenceResult = result
			}
		})
	}
}

func BenchmarkConditionSequenceEvalMetrics(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottlmetric.NewParser(ottlfuncs.StandardFuncs[ottlmetric.TransformContext](), settings, ottlmetric.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create metric parser: %v", err)
	}

	conditions := []struct {
		name       string
		predicates []string
	}{
		{name: "small", predicates: buildMetricConditions(10)},
		{name: "medium", predicates: buildMetricConditions(50)},
		{name: "large", predicates: buildMetricConditions(200)},
	}

	ctx := b.Context()

	for _, scenario := range conditions {
		parsed, err := parser.ParseConditions(scenario.predicates)
		if err != nil {
			b.Fatalf("failed to parse metric conditions: %v", err)
		}
		sequence := ottlmetric.NewConditionSequence(parsed, settings)

		contexts := make([]ottlmetric.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newBenchmarkMetricContext(len(scenario.predicates))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := sequence.Eval(ctx, contexts[i%len(contexts)])
				if err != nil {
					b.Fatalf("failed to evaluate metric conditions: %v", err)
				}
				conditionSequenceResult = result
			}
		})
	}
}

func buildMetricStatements(count int) []string {
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		switch i % 5 {
		case 0:
			result = append(result, fmt.Sprintf(`set(metric.metadata["copy_source_%[1]d"], metric.metadata["source_%[1]d"])`, i))
		case 1:
			result = append(result, fmt.Sprintf(`set(metric.metadata["formatted_%[1]d"], Concat([metric.name, metric.unit], "|"))`, i))
		case 2:
			result = append(result, fmt.Sprintf(`set(resource.attributes["metric_env_%[1]d"], resource.attributes["deployment.environment"]) where metric.is_monotonic == true`, i))
		case 3:
			result = append(result, fmt.Sprintf(`set(metric.metadata["normalized_description_%[1]d"], Trim(metric.description))`, i))
		default:
			result = append(result, fmt.Sprintf(`set(metric.metadata["owner_present_%[1]d"], IsString(metric.metadata["owner"]))`, i))
		}
	}
	return result
}

func buildMetricConditions(count int) []string {
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		switch i % 5 {
		case 0:
			result = append(result, "metric.type == METRIC_DATA_TYPE_SUM")
		case 1:
			result = append(result, "metric.aggregation_temporality == AGGREGATION_TEMPORALITY_CUMULATIVE")
		case 2:
			result = append(result, `resource.attributes["deployment.environment"] == "prod"`)
		case 3:
			result = append(result, `metric.metadata["owner"] != nil`)
		default:
			result = append(result, "metric.is_monotonic == true")
		}
	}
	return result
}

func newBenchmarkMetricContext(attributeCount int) ottlmetric.TransformContext {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	resource := resourceMetrics.Resource()
	resource.Attributes().PutStr("service.name", "benchmark-service")
	resource.Attributes().PutStr("deployment.environment", "prod")
	resource.Attributes().PutStr("cloud.region", "us-west-2")

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scope := scopeMetrics.Scope()
	scope.SetName("benchmark-scope")
	scope.SetVersion("1.0.0")
	scope.Attributes().PutStr("scope_attr", "metrics")

	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName("benchmark-metric")
	metric.SetDescription("total requests processed")
	metric.SetUnit("1")
	metric.Metadata().PutStr("owner", "team-observability")
	metric.Metadata().PutStr("slo", "p95<250ms")
	tags := metric.Metadata().PutEmptySlice("tags")
	tags.AppendEmpty().SetStr("prod")
	tags.AppendEmpty().SetStr("critical")

	sum := metric.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sum.SetIsMonotonic(true)

	dataPoints := sum.DataPoints()
	dp := dataPoints.AppendEmpty()
	dp.SetStartTimestamp(pcommon.Timestamp(1710000000000000000))
	dp.SetTimestamp(pcommon.Timestamp(1710000005000000000))
	dp.Attributes().PutStr("endpoint", "/api")
	dp.Attributes().PutStr("method", "GET")
	dp.SetIntValue(4200)

	for i := 0; i < attributeCount; i++ {
		metric.Metadata().PutStr(fmt.Sprintf("source_%d", i), fmt.Sprintf("value_%d", i))
		dp.Attributes().PutStr(fmt.Sprintf("label_%d", i), fmt.Sprintf("value_%d", i))
	}

	return ottlmetric.NewTransformContext(metric, scopeMetrics.Metrics(), scope, resource, scopeMetrics, resourceMetrics)
}

func buildLogStatements(count int) []string {
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		switch i % 5 {
		case 0:
			result = append(result, fmt.Sprintf(`set(log.attributes["conditional_copy_%[1]d"], log.attributes["source_%[1]d"]) where log.attributes["severity_text"] == "error"`, i))
		case 1:
			result = append(result, fmt.Sprintf(`set(log.attributes["sanitized_%[1]d"], ToLowerCase(log.attributes["log_message"]))`, i))
		case 2:
			result = append(result, fmt.Sprintf(`set(log.attributes["length_%[1]d"], Len(log.attributes["source_%[1]d"]))`, i))
		case 3:
			result = append(result, fmt.Sprintf(`set(resource.attributes["resource_copy_%[1]d"], log.attributes["source_%[1]d"]) where resource.attributes["deployment.environment"] == "prod"`, i))
		default:
			result = append(result, fmt.Sprintf(`set(log.attributes["has_critical_%[1]d"], ContainsValue(log.attributes["tags"], "critical"))`, i))
		}
	}
	return result
}

func buildSpanStatements(count int) []string {
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		switch i % 4 {
		case 0:
			result = append(result, fmt.Sprintf(`set(span.attributes["span_copy_%[1]d"], span.attributes["source_%[1]d"]) where span.kind.string == "SPAN_KIND_CLIENT"`, i))
		case 1:
			result = append(result, fmt.Sprintf(`set(span.attributes["status_code_%[1]d"], span.status.code)`, i))
		case 2:
			result = append(result, fmt.Sprintf(`set(resource.attributes["service_name_copy_%[1]d"], resource.attributes["service.name"])`, i))
		default:
			result = append(result, fmt.Sprintf(`set(span.attributes["trace_id_string_%[1]d"], span.trace_id.string)`, i))
		}
	}
	return result
}

func buildLogConditions(count int) []string {
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		switch i % 3 {
		case 0:
			result = append(result, fmt.Sprintf(`log.attributes["source_%[1]d"] != nil`, i))
		case 1:
			result = append(result, `ContainsValue(log.attributes["tags"], "prod")`)
		default:
			result = append(result, `resource.attributes["deployment.environment"] == "prod"`)
		}
	}
	return result
}

func newBenchmarkLogContext(attributeCount int) ottllog.TransformContext {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resource := resourceLogs.Resource()
	resource.Attributes().PutStr("service.name", "benchmark-service")
	resource.Attributes().PutStr("deployment.environment", "prod")
	resource.Attributes().PutStr("cloud.region", "us-west-2")
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scope := scopeLogs.Scope()
	scope.SetName("benchmark-scope")
	scope.SetVersion("1.0.0")
	scope.Attributes().PutStr("scope_attr", "logs")

	logRecord := scopeLogs.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.Timestamp(1710000000000000000))
	logRecord.SetObservedTimestamp(pcommon.Timestamp(1710000000000000000))
	logRecord.Attributes().PutStr("log_message", "user=alice password=secret region=us-west-2")
	logRecord.Attributes().PutStr("severity_text", "error")
	tags := logRecord.Attributes().PutEmptySlice("tags")
	tags.AppendEmpty().SetStr("prod")
	tags.AppendEmpty().SetStr("critical")

	for i := 0; i < attributeCount; i++ {
		logRecord.Attributes().PutStr(fmt.Sprintf("source_%d", i), fmt.Sprintf("value_%d", i))
	}

	logRecord.Body().SetStr("benchmark log record")

	return ottllog.NewTransformContext(logRecord, scope, resource, scopeLogs, resourceLogs)
}

func newBenchmarkSpanContext(attributeCount int) ottlspan.TransformContext {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	resource := resourceSpans.Resource()
	resource.Attributes().PutStr("service.name", "benchmark-service")
	resource.Attributes().PutStr("deployment.environment", "prod")
	resource.Attributes().PutStr("cloud.region", "us-west-2")
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	scope := scopeSpans.Scope()
	scope.SetName("benchmark-scope")
	scope.SetVersion("1.0.0")
	scope.Attributes().PutStr("scope_attr", "spans")

	span := scopeSpans.Spans().AppendEmpty()
	span.SetName("benchmark-span")
	span.SetKind(ptrace.SpanKindClient)
	span.SetTraceID(pcommon.TraceID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
	span.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8})
	span.SetStartTimestamp(pcommon.Timestamp(1710000000000000000))
	span.SetEndTimestamp(pcommon.Timestamp(1710000005000000000))
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.Status().SetMessage("ok")

	for i := 0; i < attributeCount; i++ {
		span.Attributes().PutStr(fmt.Sprintf("source_%d", i), fmt.Sprintf("span_value_%d", i))
	}

	return ottlspan.NewTransformContext(span, scope, resource, scopeSpans, resourceSpans)
}

func BenchmarkSliceToMap(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create log parser: %v", err)
	}

	stmtsNoPaths := []string{
		`set(log.attributes["mapped_no_paths"], SliceToMap(log.attributes["arr"]))`,
	}
	stmtsKeyOnly := []string{
		`set(log.attributes["mapped_key_only"], SliceToMap(log.attributes["arr"], ["id"]))`,
	}
	stmtsKeyAndValue := []string{
		`set(log.attributes["mapped_key_value"], SliceToMap(log.attributes["arr"], ["id"], ["nested","k"]))`,
	}

	parsedNoPaths, err := parser.ParseStatements(stmtsNoPaths)
	if err != nil {
		b.Fatalf("failed to parse SliceToMap no-paths statements: %v", err)
	}
	seqNoPaths := ottllog.NewStatementSequence(parsedNoPaths, settings)

	parsedKeyOnly, err := parser.ParseStatements(stmtsKeyOnly)
	if err != nil {
		b.Fatalf("failed to parse SliceToMap key-only statements: %v", err)
	}
	seqKeyOnly := ottllog.NewStatementSequence(parsedKeyOnly, settings)

	parsedKeyAndValue, err := parser.ParseStatements(stmtsKeyAndValue)
	if err != nil {
		b.Fatalf("failed to parse SliceToMap key+value statements: %v", err)
	}
	seqKeyAndValue := ottllog.NewStatementSequence(parsedKeyAndValue, settings)

	sizes := []struct {
		label string
		n     int
	}{
		{"small", 50},
		{"medium", 200},
	}

	for _, sz := range sizes {
		// no_paths: arr is slice of primitives
		b.Run("SliceToMap/no_paths/"+sz.label, func(b *testing.B) {
			ctx := b.Context()
			tc := newSliceContextWithPrimitiveArr(sz.n)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := seqNoPaths.Execute(ctx, tc); err != nil {
					b.Fatalf("execute failed: %v", err)
				}
			}
		})

		// key_only: arr is slice of maps
		b.Run("SliceToMap/key_only/"+sz.label, func(b *testing.B) {
			ctx := b.Context()
			tc := newSliceContextWithMapArr(sz.n)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := seqKeyOnly.Execute(ctx, tc); err != nil {
					b.Fatalf("execute failed: %v", err)
				}
			}
		})

		// key_and_value: arr is slice of maps
		b.Run("SliceToMap/key_and_value/"+sz.label, func(b *testing.B) {
			ctx := b.Context()
			tc := newSliceContextWithMapArr(sz.n)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := seqKeyAndValue.Execute(ctx, tc); err != nil {
					b.Fatalf("execute failed: %v", err)
				}
			}
		})
	}
}

func newSliceContextWithPrimitiveArr(arrSize int) ottllog.TransformContext {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	lr.SetTimestamp(pcommon.Timestamp(1710000000000000000))
	lr.SetObservedTimestamp(pcommon.Timestamp(1710000000000000000))

	arr := lr.Attributes().PutEmptySlice("arr")
	arr.EnsureCapacity(arrSize)
	for i := 0; i < arrSize; i++ {
		arr.AppendEmpty().SetStr("v_" + strconv.Itoa(i))
	}

	return ottllog.NewTransformContext(lr, sl.Scope(), rl.Resource(), sl, rl)
}

func newSliceContextWithMapArr(arrSize int) ottllog.TransformContext {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	lr.SetTimestamp(pcommon.Timestamp(1710000000000000000))
	lr.SetObservedTimestamp(pcommon.Timestamp(1710000000000000000))

	arr := lr.Attributes().PutEmptySlice("arr")
	arr.EnsureCapacity(arrSize)
	for i := 0; i < arrSize; i++ {
		elem := arr.AppendEmpty()
		m := elem.SetEmptyMap()
		m.PutStr("id", "item_"+strconv.Itoa(i))
		m.PutInt("val", int64(i))
		nm := m.PutEmptyMap("nested")
		nm.PutStr("k", "v_"+strconv.Itoa(i))
	}

	return ottllog.NewTransformContext(lr, sl.Scope(), rl.Resource(), sl, rl)
}
