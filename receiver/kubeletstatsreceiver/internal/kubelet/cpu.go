// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addCPUMetrics(
	mb *metadata.MetricsBuilder,
	cpuMetrics metadata.CPUMetrics,
	s *stats.CPUStats,
	currentTime pcommon.Timestamp,
	r resources,
	nodeCPULimit float64,
	calc *CPUUsageCalculator,
	cpuUsageKey string,
) {
	if s == nil {
		return
	}

	var usageCores float64
	var ok bool
	if metadata.ReceiverKubeletstatsCPUUsageScrapeBasedFeatureGate.IsEnabled() {
		usageCores, ok = calculateScrapeBasedCPUUsage(s, calc, cpuUsageKey)
	} else if s.UsageNanoCores != nil {
		usageCores, ok = float64(*s.UsageNanoCores)/1_000_000_000, true
	}
	if ok {
		cpuMetrics.Usage(mb, currentTime, usageCores)
		addCPUUtilizationMetrics(mb, cpuMetrics, usageCores, currentTime, r, nodeCPULimit)
	}

	addCPUTimeMetric(mb, cpuMetrics.Time, s, currentTime)
}

// calculateScrapeBasedCPUUsage calculates CPU usage in cores as the rate of change of
// the cumulative CPU time (UsageCoreNanoSeconds) between this scrape and the previous
// one, instead of using the value reported directly by the kubelet (UsageNanoCores).
// This is gated behind the kubeletstats.cpuUsageScrapeBased feature gate. It returns
// ok=false when usage cannot be calculated this scrape (see CPUUsageCalculator).
func calculateScrapeBasedCPUUsage(s *stats.CPUStats, calc *CPUUsageCalculator, cpuUsageKey string) (usageCores float64, ok bool) {
	if s.UsageCoreNanoSeconds == nil || s.Time.IsZero() {
		return 0, false
	}
	cpuTimeSeconds := float64(*s.UsageCoreNanoSeconds) / 1_000_000_000
	sampleTime := pcommon.NewTimestampFromTime(s.Time.Time)
	return calc.calculateUsage(cpuUsageKey, cpuTimeSeconds, sampleTime)
}

func addCPUUtilizationMetrics(
	mb *metadata.MetricsBuilder,
	cpuMetrics metadata.CPUMetrics,
	usageCores float64,
	currentTime pcommon.Timestamp,
	r resources,
	nodeCPULimit float64,
) {
	if nodeCPULimit > 0 {
		cpuMetrics.NodeUtilization(mb, currentTime, usageCores/nodeCPULimit)
	}
	if r.cpuLimit > 0 {
		cpuMetrics.LimitUtilization(mb, currentTime, usageCores/r.cpuLimit)
	}
	if r.cpuRequest > 0 {
		cpuMetrics.RequestUtilization(mb, currentTime, usageCores/r.cpuRequest)
	}
}

func addCPUTimeMetric(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordDoubleDataPointFunc, s *stats.CPUStats, currentTime pcommon.Timestamp) {
	if s.UsageCoreNanoSeconds == nil {
		return
	}
	value := float64(*s.UsageCoreNanoSeconds) / 1_000_000_000
	recordDataPoint(mb, currentTime, value)
}
