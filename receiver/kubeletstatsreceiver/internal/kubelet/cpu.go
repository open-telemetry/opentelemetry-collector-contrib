// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/precision"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

const (
	nanoCoresPerCore  = 1_000_000_000
	milliCoresPerCore = 1_000
	nanoCoresPerMilli = nanoCoresPerCore / milliCoresPerCore
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
	utilization := floatUtilization
	if metadata.ReceiverKubeletStatsCPUUtilizationRoundingFeatureGate.IsEnabled() {
		utilization = roundedUtilization
	}
	if nodeCPULimit > 0 {
		cpuMetrics.NodeUtilization(mb, currentTime, utilization(usageCores, nodeCPULimit))
	}
	if r.cpuLimit > 0 {
		cpuMetrics.LimitUtilization(mb, currentTime, utilization(usageCores, r.cpuLimit))
	}
	if r.cpuRequest > 0 {
		cpuMetrics.RequestUtilization(mb, currentTime, utilization(usageCores, r.cpuRequest))
	}
}

func floatUtilization(usageCores, limitCores float64) float64 {
	return usageCores / limitCores
}

// roundedUtilization computes usageCores/limitCores from integer nanocore values and
// rounds the result to the number of significant digits justified by the inputs
// (see precision.Ratio), avoiding float64 artifacts such as 0.30000000000000004.
//
// Converting back to integers recovers the original values for the inputs used
// here, because the rounding only undoes float64 representation error: CPU limits,
// requests and node capacity originate from Kubernetes resource.Quantity values,
// which have millicore resolution, and usage originates from integer nanocore or
// nanosecond values reported by the kubelet. The final result is still rounded by
// precision.Ratio, so it may differ slightly from the exact float64 quotient.
func roundedUtilization(usageCores, limitCores float64) float64 {
	usageNanoCores := uint64(math.Round(usageCores * nanoCoresPerCore))
	limitNanoCores := uint64(math.Round(limitCores*milliCoresPerCore)) * nanoCoresPerMilli
	return precision.Ratio(usageNanoCores, limitNanoCores)
}

func addCPUTimeMetric(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordDoubleDataPointFunc, s *stats.CPUStats, currentTime pcommon.Timestamp) {
	if s.UsageCoreNanoSeconds == nil {
		return
	}
	value := float64(*s.UsageCoreNanoSeconds) / 1_000_000_000
	recordDataPoint(mb, currentTime, value)
}
