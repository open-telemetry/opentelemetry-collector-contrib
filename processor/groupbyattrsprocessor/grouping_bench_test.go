// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor/internal/metadata"
)

const groupingKey = "host.name"

func fillBenchResource(res pcommon.Resource, attrCount int) {
	attrs := res.Attributes()
	attrs.EnsureCapacity(attrCount)
	for i := range attrCount {
		attrs.PutStr(fmt.Sprint("resource.attr.", i), fmt.Sprint("resource-value-", i))
	}
}

func benchLogs(recordCount, groups, resourceAttrCount int) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	fillBenchResource(rl.Resource(), resourceAttrCount)
	sl := rl.ScopeLogs().AppendEmpty()
	for i := range recordCount {
		lr := sl.LogRecords().AppendEmpty()
		lr.Body().SetStr("log message")
		lr.Attributes().PutStr(groupingKey, fmt.Sprint("host-", i%groups))
		lr.Attributes().PutStr("log.attr", "some-value")
	}
	return ld
}

func benchTraces(spanCount, groups, resourceAttrCount int) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	fillBenchResource(rs.Resource(), resourceAttrCount)
	ss := rs.ScopeSpans().AppendEmpty()
	for i := range spanCount {
		span := ss.Spans().AppendEmpty()
		span.SetName("span")
		span.Attributes().PutStr(groupingKey, fmt.Sprint("host-", i%groups))
		span.Attributes().PutStr("span.attr", "some-value")
	}
	return td
}

var benchCases = []struct {
	groups            int
	resourceAttrCount int
}{
	{groups: 1, resourceAttrCount: 15},
	{groups: 10, resourceAttrCount: 15},
	{groups: 100, resourceAttrCount: 15},
	// Few resource attributes: the floor, where the merge costs least.
	{groups: 10, resourceAttrCount: 2},
}

const benchRecordCount = 1000

func BenchmarkGroupByAttrsLogs(bb *testing.B) {
	for _, bc := range benchCases {
		bb.Run(fmt.Sprintf("groups=%d/resource_attrs=%d", bc.groups, bc.resourceAttrCount), func(b *testing.B) {
			gap, err := createGroupByAttrsProcessor(processortest.NewNopSettings(metadata.Type), []string{groupingKey})
			require.NoError(b, err)
			template := benchLogs(benchRecordCount, bc.groups, bc.resourceAttrCount)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				// The input is mutated, so rebuild it each iteration.
				b.StopTimer()
				ld := plog.NewLogs()
				template.CopyTo(ld)
				b.StartTimer()

				if _, err := gap.processLogs(b.Context(), ld); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkGroupByAttrsTraces(bb *testing.B) {
	for _, bc := range benchCases {
		bb.Run(fmt.Sprintf("groups=%d/resource_attrs=%d", bc.groups, bc.resourceAttrCount), func(b *testing.B) {
			gap, err := createGroupByAttrsProcessor(processortest.NewNopSettings(metadata.Type), []string{groupingKey})
			require.NoError(b, err)
			template := benchTraces(benchRecordCount, bc.groups, bc.resourceAttrCount)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				b.StopTimer()
				td := ptrace.NewTraces()
				template.CopyTo(td)
				b.StartTimer()

				if _, err := gap.processTraces(b.Context(), td); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
