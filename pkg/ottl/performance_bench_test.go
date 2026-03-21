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
	"go.opentelemetry.io/collector/pdata/xpdata/entity"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
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

	logParser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create log parser: %v", err)
	}

	spanParser, err := ottlspan.NewParser(ottlfuncs.StandardFuncs[*ottlspan.TransformContext](), settings, ottlspan.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create span parser: %v", err)
	}

	metricParser, err := ottlmetric.NewParser(ottlfuncs.StandardFuncs[*ottlmetric.TransformContext](), settings, ottlmetric.EnablePathContextNames())
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
			for b.Loop() {
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
			for b.Loop() {
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
			for b.Loop() {
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
	parser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
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

		contexts := make([]*ottllog.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newBenchmarkLogContext(len(scenario.statements))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				if err := sequence.Execute(ctx, contexts[i%len(contexts)]); err != nil {
					b.Fatalf("failed to execute log statements: %v", err)
				}
			}
		})

		for i := range contexts {
			contexts[i].Close()
		}
	}
}

func BenchmarkStatementSequenceExecuteSpans(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottlspan.NewParser(ottlfuncs.StandardFuncs[*ottlspan.TransformContext](), settings, ottlspan.EnablePathContextNames())
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

		contexts := make([]*ottlspan.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newBenchmarkSpanContext(len(scenario.statements))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				if err := sequence.Execute(ctx, contexts[i%len(contexts)]); err != nil {
					b.Fatalf("failed to execute span statements: %v", err)
				}
			}
		})
		for i := range contexts {
			contexts[i].Close()
		}
	}
}

func BenchmarkStatementSequenceExecuteMetrics(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottlmetric.NewParser(ottlfuncs.StandardFuncs[*ottlmetric.TransformContext](), settings, ottlmetric.EnablePathContextNames())
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

		contexts := make([]*ottlmetric.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newBenchmarkMetricContext(len(scenario.statements))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				if err := sequence.Execute(ctx, contexts[i%len(contexts)]); err != nil {
					b.Fatalf("failed to execute metric statements: %v", err)
				}
			}
		})

		for i := range contexts {
			contexts[i].Close()
		}
	}
}

func BenchmarkStatementExecute_EntityRefsSync(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	resParser, err := ottlresource.NewParser(ottlfuncs.StandardFuncs[*ottlresource.TransformContext](), settings, ottlresource.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create resource parser: %v", err)
	}
	logParser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create log parser: %v", err)
	}
	statements := []string{
		`delete_key(resource.attributes, "host.type")`,    // entityRef-DescKey
		`delete_key(resource.attributes, "service.name")`, // entityRef-IdKey
	}
	scenarios := []struct {
		name       string
		attrsCount int
	}{
		{name: "small", attrsCount: 10},
		{name: "medium", attrsCount: 20},
		{name: "large", attrsCount: 30},
	}

	ctx := b.Context()
	for _, scenario := range scenarios {
		resParsed, err := resParser.ParseStatements(statements)
		if err != nil {
			b.Fatalf("failed to parse: %v", err)
		}
		logParsed, err := logParser.ParseStatements(statements)
		if err != nil {
			b.Fatalf("failed to parse: %v", err)
		}

		resSequence := ottlresource.NewStatementSequence(resParsed, settings)
		logSequence := ottllog.NewStatementSequence(logParsed, settings)

		resContexts := make([]*ottlresource.TransformContext, benchmarkContextPoolSize)
		for i := range resContexts {
			resContexts[i] = newBenchmarkResourceContext(scenario.attrsCount)
		}
		logContexts := make([]*ottllog.TransformContext, benchmarkContextPoolSize)
		for i := range logContexts {
			logContexts[i] = newBenchmarkLogContext(scenario.attrsCount)
			resContexts[i].GetResource().CopyTo(logContexts[i].GetResource())
		}

		b.Run(scenario.name+"_resourceContext", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				if err := resSequence.Execute(ctx, resContexts[i%len(resContexts)]); err != nil {
					b.Fatalf("failed to execute log statements: %v", err)
				}
			}
		})
		for i := range resContexts {
			resContexts[i].Close()
		}

		b.Run(scenario.name+"_logContext", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				if err := logSequence.Execute(ctx, logContexts[i%len(logContexts)]); err != nil {
					b.Fatalf("failed to execute log statements: %v", err)
				}
			}
		})
		for i := range logContexts {
			logContexts[i].Close()
		}
	}
}

func BenchmarkConditionSequenceEvalLogs(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
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

		contexts := make([]*ottllog.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newBenchmarkLogContext(len(scenario.predicates))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				result, err := sequence.Eval(ctx, contexts[i%len(contexts)])
				if err != nil {
					b.Fatalf("failed to evaluate log conditions: %v", err)
				}
				conditionSequenceResult = result
			}
		})
		for i := range contexts {
			contexts[i].Close()
		}
	}
}

func BenchmarkConditionSequenceEvalMetrics(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottlmetric.NewParser(ottlfuncs.StandardFuncs[*ottlmetric.TransformContext](), settings, ottlmetric.EnablePathContextNames())
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

		contexts := make([]*ottlmetric.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newBenchmarkMetricContext(len(scenario.predicates))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				result, err := sequence.Eval(ctx, contexts[i%len(contexts)])
				if err != nil {
					b.Fatalf("failed to evaluate metric conditions: %v", err)
				}
				conditionSequenceResult = result
			}
		})
		for i := range contexts {
			contexts[i].Close()
		}
	}
}

func buildMetricStatements(count int) []string {
	result := make([]string, 0, count)
	for i := range count {
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
	for i := range count {
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

func newBenchmarkMetricContext(attributeCount int) *ottlmetric.TransformContext {
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

	for i := range attributeCount {
		metric.Metadata().PutStr(fmt.Sprintf("source_%d", i), fmt.Sprintf("value_%d", i))
		dp.Attributes().PutStr(fmt.Sprintf("label_%d", i), fmt.Sprintf("value_%d", i))
	}

	return ottlmetric.NewTransformContextPtr(resourceMetrics, scopeMetrics, metric)
}

func buildLogStatements(count int) []string {
	result := make([]string, 0, count)
	for i := range count {
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

func buildResourceAttributesWithEntityRefs(attrsCount int) pcommon.Resource {
	attrs := []struct {
		key   string
		value string
	}{
		{string(semconv.HostNameKey), "bench-host-01"},
		{string(semconv.HostIDKey), "i-1234567890abcdef0"},
		{string(semconv.HostTypeKey), "m5.xlarge"},
		{string(semconv.HostArchKey), "amd64"},
		{string(semconv.HostImageNameKey), "trixie"},

		{string(semconv.ServiceNameKey), "bench-service"},
		{string(semconv.ServiceInstanceIDKey), "instance-abc"},
		{string(semconv.ServiceNamespaceKey), "foo"},
		{string(semconv.ServiceVersionKey), "1.4.2"},

		{string(semconv.K8SPodNameKey), "bench-pod-xyz"},
		{string(semconv.K8SPodUIDKey), "uid-123-abc"},
		{string(semconv.K8SNodeNameKey), "node-01"},
		{string(semconv.K8SNamespaceNameKey), "bench"},
		{string(semconv.K8SClusterNameKey), "bench-cluster"},

		{string(semconv.CloudAccountIDKey), "123456789"},
		{string(semconv.CloudProviderKey), "bar"},
		{string(semconv.CloudRegionKey), "us-west-2"},
		{string(semconv.CloudAvailabilityZoneKey), "us-east-1a"},
		{string(semconv.CloudPlatformKey), "bar_ec2"},

		{string(semconv.ContainerNameKey), "bench-container"},
		{string(semconv.ContainerIDKey), "abc123def456"},
		{string(semconv.ContainerImageNameKey), "bench-img"},
		{string(semconv.ContainerImageTagsKey), "v1.2.3"},

		{string(semconv.ProcessPIDKey), "1234"},
		{string(semconv.ProcessExecutableNameKey), "bench-process"},
		{string(semconv.ProcessExecutablePathKey), "/usr/bin/bench-process"},
		{string(semconv.ProcessCommandKey), "bench-process --config /etc/bench.yaml"},
		{string(semconv.ProcessOwnerKey), "bench-user"},

		{string(semconv.OSTypeKey), "linux"},
		{string(semconv.OSVersionKey), "6.1.0-debian"},
	}

	resourceAttrs := pcommon.NewResource()

	limit := attrsCount
	if limit > len(attrs) {
		limit = len(attrs)
	}

	for _, attr := range attrs[:limit] {
		resourceAttrs.Attributes().PutStr(attr.key, attr.value)
	}

	addEntityRefsToResource(resourceAttrs)

	return resourceAttrs
}

func addEntityRefsToResource(res pcommon.Resource) {
	entityRefs := entity.ResourceEntityRefs(res)
	resAttrs := res.Attributes()

	if _, ok := resAttrs.Get(string(semconv.HostNameKey)); ok {
		ref := entityRefs.AppendEmpty()
		ref.SetType("host")
		ref.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
		ref.IdKeys().Append(string(semconv.HostNameKey))
		ref.IdKeys().Append(string(semconv.HostIDKey))
		ref.DescriptionKeys().Append(string(semconv.HostTypeKey))
		ref.DescriptionKeys().Append(string(semconv.HostArchKey))
		ref.DescriptionKeys().Append(string(semconv.HostImageNameKey))
	}

	if _, ok := resAttrs.Get(string(semconv.ServiceNameKey)); ok {
		ref := entityRefs.AppendEmpty()
		ref.SetType("service")
		ref.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
		ref.IdKeys().Append(string(semconv.ServiceNameKey))
		ref.IdKeys().Append(string(semconv.ServiceInstanceIDKey))
		ref.IdKeys().Append(string(semconv.ServiceNamespaceKey))
		ref.DescriptionKeys().Append(string(semconv.ServiceVersionKey))
	}

	if _, ok := resAttrs.Get(string(semconv.K8SPodNameKey)); ok {
		ref := entityRefs.AppendEmpty()
		ref.SetType("k8s.pod")
		ref.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
		ref.IdKeys().Append(string(semconv.K8SPodNameKey))
		ref.IdKeys().Append(string(semconv.K8SPodUIDKey))
		ref.IdKeys().Append(string(semconv.K8SNodeNameKey))
		ref.IdKeys().Append(string(semconv.K8SNamespaceNameKey))
		ref.IdKeys().Append(string(semconv.K8SClusterNameKey))
	}

	if _, ok := resAttrs.Get(string(semconv.CloudAccountIDKey)); ok {
		ref := entityRefs.AppendEmpty()
		ref.SetType("cloud")
		ref.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
		ref.IdKeys().Append(string(semconv.CloudAccountIDKey))
		ref.IdKeys().Append(string(semconv.CloudProviderKey))
		ref.DescriptionKeys().Append(string(semconv.CloudRegionKey))
		ref.DescriptionKeys().Append(string(semconv.CloudAvailabilityZoneKey))
		ref.DescriptionKeys().Append(string(semconv.CloudPlatformKey))
	}

	if _, ok := resAttrs.Get(string(semconv.ContainerNameKey)); ok {
		ref := entityRefs.AppendEmpty()
		ref.SetType("container")
		ref.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
		ref.IdKeys().Append(string(semconv.ContainerNameKey))
		ref.IdKeys().Append(string(semconv.ContainerIDKey))
		ref.DescriptionKeys().Append(string(semconv.ContainerImageNameKey))
		ref.DescriptionKeys().Append(string(semconv.ContainerImageTagsKey))
	}

	if _, ok := resAttrs.Get(string(semconv.ProcessPIDKey)); ok {
		ref := entityRefs.AppendEmpty()
		ref.SetType("process")
		ref.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
		ref.IdKeys().Append(string(semconv.ProcessPIDKey))
		ref.IdKeys().Append(string(semconv.ProcessExecutableNameKey))
		ref.DescriptionKeys().Append(string(semconv.ProcessExecutablePathKey))
		ref.DescriptionKeys().Append(string(semconv.ProcessCommandKey))
		ref.DescriptionKeys().Append(string(semconv.ProcessOwnerKey))
	}

	if _, ok := resAttrs.Get(string(semconv.OSTypeKey)); ok {
		ref := entityRefs.AppendEmpty()
		ref.SetType("os")
		ref.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
		ref.IdKeys().Append(string(semconv.OSTypeKey))
		ref.DescriptionKeys().Append(string(semconv.OSVersionKey))
	}
}

func buildSpanStatements(count int) []string {
	result := make([]string, 0, count)
	for i := range count {
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
	for i := range count {
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

func newBenchmarkResourceContext(attributeCount int) *ottlresource.TransformContext {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	res := buildResourceAttributesWithEntityRefs(attributeCount)
	res.MoveTo(resourceLogs.Resource())

	return ottlresource.NewTransformContextPtr(resourceLogs.Resource(), resourceLogs)
}

func newBenchmarkLogContext(attributeCount int) *ottllog.TransformContext {
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

	for i := range attributeCount {
		logRecord.Attributes().PutStr(fmt.Sprintf("source_%d", i), fmt.Sprintf("value_%d", i))
	}

	logRecord.Body().SetStr("benchmark log record")

	return ottllog.NewTransformContextPtr(resourceLogs, scopeLogs, logRecord)
}

func newBenchmarkSpanContext(attributeCount int) *ottlspan.TransformContext {
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

	for i := range attributeCount {
		span.Attributes().PutStr(fmt.Sprintf("source_%d", i), fmt.Sprintf("span_value_%d", i))
	}

	return ottlspan.NewTransformContextPtr(resourceSpans, scopeSpans, span)
}

func BenchmarkSliceToMap(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
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
			for b.Loop() {
				if err := seqNoPaths.Execute(ctx, tc); err != nil {
					b.Fatalf("execute failed: %v", err)
				}
			}
			tc.Close()
		})

		// key_only: arr is slice of maps
		b.Run("SliceToMap/key_only/"+sz.label, func(b *testing.B) {
			ctx := b.Context()
			tc := newSliceContextWithMapArr(sz.n)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := seqKeyOnly.Execute(ctx, tc); err != nil {
					b.Fatalf("execute failed: %v", err)
				}
			}
			tc.Close()
		})

		// key_and_value: arr is slice of maps
		b.Run("SliceToMap/key_and_value/"+sz.label, func(b *testing.B) {
			ctx := b.Context()
			tc := newSliceContextWithMapArr(sz.n)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := seqKeyAndValue.Execute(ctx, tc); err != nil {
					b.Fatalf("execute failed: %v", err)
				}
			}
			tc.Close()
		})
	}
}

func newSliceContextWithPrimitiveArr(arrSize int) *ottllog.TransformContext {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	lr.SetTimestamp(pcommon.Timestamp(1710000000000000000))
	lr.SetObservedTimestamp(pcommon.Timestamp(1710000000000000000))

	arr := lr.Attributes().PutEmptySlice("arr")
	arr.EnsureCapacity(arrSize)
	for i := range arrSize {
		arr.AppendEmpty().SetStr("v_" + strconv.Itoa(i))
	}

	return ottllog.NewTransformContextPtr(rl, sl, lr)
}

func newSliceContextWithMapArr(arrSize int) *ottllog.TransformContext {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	lr.SetTimestamp(pcommon.Timestamp(1710000000000000000))
	lr.SetObservedTimestamp(pcommon.Timestamp(1710000000000000000))

	arr := lr.Attributes().PutEmptySlice("arr")
	arr.EnsureCapacity(arrSize)
	for i := range arrSize {
		elem := arr.AppendEmpty()
		m := elem.SetEmptyMap()
		m.PutStr("id", "item_"+strconv.Itoa(i))
		m.PutInt("val", int64(i))
		nm := m.PutEmptyMap("nested")
		nm.PutStr("k", "v_"+strconv.Itoa(i))
	}

	return ottllog.NewTransformContextPtr(rl, sl, lr)
}
