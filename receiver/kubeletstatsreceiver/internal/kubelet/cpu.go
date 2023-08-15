// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addCPUMetrics(mb *metadata.MetricsBuilder, cpuMetrics metadata.CPUMetrics, s *stats.CPUStats, currentTime pcommon.Timestamp, limit *float64) {
	if s == nil {
		return
	}
	addCPUUsageMetric(mb, cpuMetrics.Utilization, s, currentTime)
	addCPUTimeMetric(mb, cpuMetrics.Time, s, currentTime)

	if limit != nil {
		addCPUUsagePercentMetric(mb, cpuMetrics.UsagePercent, s, currentTime, *limit)
	}
}

func addCPUUsageMetric(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordDoubleDataPointFunc, s *stats.CPUStats, currentTime pcommon.Timestamp) {
	if s.UsageNanoCores == nil {
		return
	}
	value := float64(*s.UsageNanoCores) / 1_000_000_000
	recordDataPoint(mb, currentTime, value)
}

func addCPUTimeMetric(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordDoubleDataPointFunc, s *stats.CPUStats, currentTime pcommon.Timestamp) {
	if s.UsageCoreNanoSeconds == nil {
		return
	}
	value := float64(*s.UsageCoreNanoSeconds) / 1_000_000_000
	recordDataPoint(mb, currentTime, value)
}

func addCPUUsagePercentMetric(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordDoubleDataPointFunc, s *stats.CPUStats, currentTime pcommon.Timestamp, limit float64) {
	if s.UsageNanoCores == nil || limit <= 0 {
		return
	}
	cpuUsage := float64(*s.UsageNanoCores) / 1_000_000_000
	value := (cpuUsage / limit) * 100
	recordDataPoint(mb, currentTime, value)
}
