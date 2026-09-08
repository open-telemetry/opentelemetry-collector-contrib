// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl_test

import (
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func buildLikeGetterStatements(count int) []string {
	result := make([]string, 0, count)
	for i := range count {
		switch i % 5 {
		case 0:
			result = append(result, fmt.Sprintf(`set(log.attributes["int_%[1]d"], Int(log.attributes["num_str"]))`, i))
		case 1:
			result = append(result, fmt.Sprintf(`set(log.attributes["str_%[1]d"], String(log.attributes["num_int"]))`, i))
		case 2:
			result = append(result, fmt.Sprintf(`set(log.attributes["dbl_%[1]d"], Double(log.attributes["num_str"]))`, i))
		case 3:
			result = append(result, fmt.Sprintf(`set(log.attributes["bool_%[1]d"], Bool(log.attributes["num_int"]))`, i))
		default:
			result = append(result, fmt.Sprintf(`set(log.attributes["cat_%[1]d"], Concat([log.attributes["a"], log.attributes["b"]], "-"))`, i))
		}
	}
	return result
}

func newLikeGetterLogContext(attributeCount int) *ottllog.TransformContext {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	logRecord.Attributes().PutStr("num_str", "12345")
	logRecord.Attributes().PutInt("num_int", 67890)
	logRecord.Attributes().PutStr("a", "alpha")
	logRecord.Attributes().PutStr("b", "beta")
	for i := range attributeCount {
		logRecord.Attributes().PutStr(fmt.Sprintf("source_%d", i), fmt.Sprintf("value_%d", i))
	}
	return ottllog.NewTransformContextPtr(resourceLogs, scopeLogs, logRecord)
}

func BenchmarkLikeGetterStatements(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create parser: %v", err)
	}

	scenarios := []struct {
		name       string
		statements []string
	}{
		{name: "small", statements: buildLikeGetterStatements(10)},
		{name: "medium", statements: buildLikeGetterStatements(50)},
		{name: "large", statements: buildLikeGetterStatements(200)},
	}

	ctx := b.Context()

	for _, scenario := range scenarios {
		parsed, err := parser.ParseStatements(scenario.statements)
		if err != nil {
			b.Fatalf("failed to parse statements: %v", err)
		}
		sequence := ottllog.NewStatementSequence(parsed, settings)

		contexts := make([]*ottllog.TransformContext, benchmarkContextPoolSize)
		for i := range contexts {
			contexts[i] = newLikeGetterLogContext(len(scenario.statements))
		}

		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				if err := sequence.Execute(ctx, contexts[i%len(contexts)]); err != nil {
					b.Fatalf("failed to execute statements: %v", err)
				}
			}
		})
	}
}
