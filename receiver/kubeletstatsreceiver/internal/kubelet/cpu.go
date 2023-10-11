// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addCPUMetrics(mb *metadata.MetricsBuilder, cpuMetrics metadata.CPUMetrics, s *stats.CPUStats, currentTime pcommon.Timestamp, r resources) {
	if s == nil {
		return
	}
	addCPUUsageMetric(mb, cpuMetrics, s, currentTime, r)
	addCPUTimeMetric(mb, cpuMetrics.Time, s, currentTime)
}

func addCPUUsageMetric(mb *metadata.MetricsBuilder, cpuMetrics metadata.CPUMetrics, s *stats.CPUStats, currentTime pcommon.Timestamp, r resources) {
	if s.UsageNanoCores == nil {
		return
	}
	value := float64(*s.UsageNanoCores) / 1_000_000_000
	cpuMetrics.Utilization(mb, currentTime, value)

	if r.cpuLimit > 0 {
		cpuMetrics.LimitUtilization(mb, currentTime, value/r.cpuLimit)
	}
	if r.cpuRequest > 0 {
		cpuMetrics.RequestUtilization(mb, currentTime, value/r.cpuRequest)
	}
}

func addCPUTimeMetric(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordDoubleDataPointFunc, s *stats.CPUStats, currentTime pcommon.Timestamp) {
	if s.UsageCoreNanoSeconds == nil {
		return
	}
	value := float64(*s.UsageCoreNanoSeconds) / 1_000_000_000
	recordDataPoint(mb, currentTime, value)
}
