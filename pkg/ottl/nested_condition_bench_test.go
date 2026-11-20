// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl_test

import (
	"fmt"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func buildNestedCondition(depth int) string {
	return strings.Repeat("(", depth) + `log.body == "x"` + strings.Repeat(")", depth)
}

func buildNestedStatement(depth int) string {
	return `set(log.body, "y") where ` + buildNestedCondition(depth)
}

func BenchmarkNestedConditionParseConditions(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create log parser: %v", err)
	}

	depths := []int{100, 300, 600, 1000, 2000}

	for _, d := range depths {
		cond := buildNestedCondition(d)
		b.Run(fmt.Sprintf("depth=%d", d), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, err := parser.ParseConditions([]string{cond})
				if err != nil {
					b.Fatalf("failed to parse condition at depth %d: %v", d, err)
				}
			}
		})
	}
}

func BenchmarkNestedConditionParseStatements(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create log parser: %v", err)
	}

	depths := []int{100, 300, 600, 1000, 2000}

	for _, d := range depths {
		stmt := buildNestedStatement(d)
		b.Run(fmt.Sprintf("depth=%d", d), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, err := parser.ParseStatements([]string{stmt})
				if err != nil {
					b.Fatalf("failed to parse statement at depth %d: %v", d, err)
				}
			}
		})
	}
}

func BenchmarkNestedConditionRejectedValidation(b *testing.B) {
	settings := componenttest.NewNopTelemetrySettings()
	parser, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	if err != nil {
		b.Fatalf("failed to create log parser: %v", err)
	}

	// Benchmark the rejection validation speed for depth 2001 and 10000
	for _, d := range []int{2001, 10000} {
		cond := buildNestedCondition(d)
		b.Run(fmt.Sprintf("depth=%d", d), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, err := parser.ParseConditions([]string{cond})
				if err == nil {
					b.Fatalf("expected error for depth %d", d)
				}
			}
		})
	}
}
