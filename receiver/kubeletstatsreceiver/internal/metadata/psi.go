// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"

import "go.opentelemetry.io/collector/pdata/pcommon"

// RecordDoublePSIAvgFunc records a double gauge PSI stall-average data point
// with both pressure.type and pressure.window attributes.
type RecordDoublePSIAvgFunc func(*MetricsBuilder, pcommon.Timestamp, float64, AttributePressureType, AttributePressureWindow)

// RecordIntPSITotalFunc records an int64 cumulative PSI stall-total data point
// with a pressure.type attribute.
type RecordIntPSITotalFunc func(*MetricsBuilder, pcommon.Timestamp, int64, AttributePressureType)

// PSIMetrics holds the record-function pair for one PSI resource (CPU, memory, or IO)
// at a particular scope (node, pod, or container).
type PSIMetrics struct {
	Avg   RecordDoublePSIAvgFunc
	Total RecordIntPSITotalFunc
}

// --- Node PSI metric vars ---

// NodeCPUPressureMetrics is the PSIMetrics dispatch table for k8s.node.cpu.pressure.{avg,total}.
var NodeCPUPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sNodeCPUPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sNodeCPUPressureTotalDataPoint,
}

// NodeMemoryPressureMetrics is the PSIMetrics dispatch table for k8s.node.memory.pressure.{avg,total}.
var NodeMemoryPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sNodeMemoryPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sNodeMemoryPressureTotalDataPoint,
}

// NodeIOPressureMetrics is the PSIMetrics dispatch table for k8s.node.io.pressure.{avg,total}.
var NodeIOPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sNodeIoPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sNodeIoPressureTotalDataPoint,
}

// --- Pod PSI metric vars ---

// PodCPUPressureMetrics is the PSIMetrics dispatch table for k8s.pod.cpu.pressure.{avg,total}.
var PodCPUPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sPodCPUPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sPodCPUPressureTotalDataPoint,
}

// PodMemoryPressureMetrics is the PSIMetrics dispatch table for k8s.pod.memory.pressure.{avg,total}.
var PodMemoryPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sPodMemoryPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sPodMemoryPressureTotalDataPoint,
}

// PodIOPressureMetrics is the PSIMetrics dispatch table for k8s.pod.io.pressure.{avg,total}.
var PodIOPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sPodIoPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sPodIoPressureTotalDataPoint,
}

// --- Container PSI metric vars ---

// ContainerCPUPressureMetrics is the PSIMetrics dispatch table for container.cpu.pressure.{avg,total}.
var ContainerCPUPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordContainerCPUPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordContainerCPUPressureTotalDataPoint,
}

// ContainerMemoryPressureMetrics is the PSIMetrics dispatch table for container.memory.pressure.{avg,total}.
var ContainerMemoryPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordContainerMemoryPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordContainerMemoryPressureTotalDataPoint,
}

// ContainerIOPressureMetrics is the PSIMetrics dispatch table for container.io.pressure.{avg,total}.
var ContainerIOPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordContainerIoPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordContainerIoPressureTotalDataPoint,
}
