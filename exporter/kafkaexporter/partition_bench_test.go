// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// generateLogsWithResources creates a plog.Logs with n distinct ResourceLogs,
// each containing 1 ScopeLogs with 5 LogRecords. Resource attributes are
// unique per resource to simulate high-cardinality partitioning.
func generateLogsWithResources(n int) plog.Logs {
	ld := plog.NewLogs()
	for i := range n {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", fmt.Sprintf("svc-%d", i))
		rl.Resource().Attributes().PutStr("host.name", fmt.Sprintf("host-%d", i))
		sl := rl.ScopeLogs().AppendEmpty()
		for range 5 {
			lr := sl.LogRecords().AppendEmpty()
			lr.Body().SetStr("log message body")
			lr.Attributes().PutStr("key", "value")
			lr.SetTimestamp(pcommon.Timestamp(1_000_000_000))
		}
	}
	return ld
}

// generateMetricsWithResources creates a pmetric.Metrics with n distinct
// ResourceMetrics, each containing 1 ScopeMetrics with 3 Gauge data points.
func generateMetricsWithResources(n int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for i := range n {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("service.name", fmt.Sprintf("svc-%d", i))
		rm.Resource().Attributes().PutStr("host.name", fmt.Sprintf("host-%d", i))
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("test.metric")
		g := m.SetEmptyGauge()
		for range 3 {
			dp := g.DataPoints().AppendEmpty()
			dp.SetDoubleValue(42.0)
			dp.Attributes().PutStr("key", "value")
		}
	}
	return md
}

func BenchmarkPartitionLogsByResourceAttributes(b *testing.B) {
	for _, numResources := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("resources=%d", numResources), func(b *testing.B) {
			ld := generateLogsWithResources(numResources)
			messenger := &kafkaLogsMessenger{
				config: Config{PartitionLogsByResourceAttributes: true},
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for key, data := range messenger.partitionData(ld) {
					_ = key
					_ = data
				}
			}
		})
	}
}

func BenchmarkPartitionMetricsByResourceAttributes(b *testing.B) {
	for _, numResources := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("resources=%d", numResources), func(b *testing.B) {
			md := generateMetricsWithResources(numResources)
			messenger := &kafkaMetricsMessenger{
				config: Config{PartitionMetricsByResourceAttributes: true},
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for key, data := range messenger.partitionData(md) {
					_ = key
					_ = data
				}
			}
		})
	}
}
