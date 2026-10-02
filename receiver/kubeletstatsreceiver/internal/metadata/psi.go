// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"

import "go.opentelemetry.io/collector/pdata/pcommon"

// RecordDoublePSIAvgFunc records a double gauge PSI stall-average data point
// with both psi.type and psi.window attributes.
type RecordDoublePSIAvgFunc func(*MetricsBuilder, pcommon.Timestamp, float64, AttributePsiType, AttributePsiWindow)

// RecordDoublePSITimeFunc records a float64 cumulative PSI stall-time data point
// with a psi.type attribute. The value is in seconds.
type RecordDoublePSITimeFunc func(*MetricsBuilder, pcommon.Timestamp, float64, AttributePsiType)

// PSIMetrics holds the record-function pair for one PSI resource (CPU, memory, or IO)
// at a particular scope (node, pod, or container).
type PSIMetrics struct {
	Avg  RecordDoublePSIAvgFunc
	Time RecordDoublePSITimeFunc
}

// --- Node PSI metric vars ---

// NodeCPUPressureMetrics is the PSIMetrics dispatch table for k8s.node.cpu.pressure.{avg,time}.
var NodeCPUPressureMetrics = PSIMetrics{
	Avg:  (*MetricsBuilder).RecordK8sNodeCPUPressureAvgDataPoint,
	Time: (*MetricsBuilder).RecordK8sNodeCPUPressureTimeDataPoint,
}

// NodeMemoryPressureMetrics is the PSIMetrics dispatch table for k8s.node.memory.pressure.{avg,time}.
var NodeMemoryPressureMetrics = PSIMetrics{
	Avg:  (*MetricsBuilder).RecordK8sNodeMemoryPressureAvgDataPoint,
	Time: (*MetricsBuilder).RecordK8sNodeMemoryPressureTimeDataPoint,
}

// NodeIOPressureMetrics is the PSIMetrics dispatch table for k8s.node.io.pressure.{avg,time}.
var NodeIOPressureMetrics = PSIMetrics{
	Avg:  (*MetricsBuilder).RecordK8sNodeIoPressureAvgDataPoint,
	Time: (*MetricsBuilder).RecordK8sNodeIoPressureTimeDataPoint,
}

// --- Pod PSI metric vars ---

// PodCPUPressureMetrics is the PSIMetrics dispatch table for k8s.pod.cpu.pressure.{avg,time}.
var PodCPUPressureMetrics = PSIMetrics{
	Avg:  (*MetricsBuilder).RecordK8sPodCPUPressureAvgDataPoint,
	Time: (*MetricsBuilder).RecordK8sPodCPUPressureTimeDataPoint,
}

// PodMemoryPressureMetrics is the PSIMetrics dispatch table for k8s.pod.memory.pressure.{avg,time}.
var PodMemoryPressureMetrics = PSIMetrics{
	Avg:  (*MetricsBuilder).RecordK8sPodMemoryPressureAvgDataPoint,
	Time: (*MetricsBuilder).RecordK8sPodMemoryPressureTimeDataPoint,
}

// PodIOPressureMetrics is the PSIMetrics dispatch table for k8s.pod.io.pressure.{avg,time}.
var PodIOPressureMetrics = PSIMetrics{
	Avg:  (*MetricsBuilder).RecordK8sPodIoPressureAvgDataPoint,
	Time: (*MetricsBuilder).RecordK8sPodIoPressureTimeDataPoint,
}

// --- Container PSI metric vars ---

// ContainerCPUPressureMetrics is the PSIMetrics dispatch table for container.cpu.pressure.{avg,time}.
var ContainerCPUPressureMetrics = PSIMetrics{
	Avg:  (*MetricsBuilder).RecordContainerCPUPressureAvgDataPoint,
	Time: (*MetricsBuilder).RecordContainerCPUPressureTimeDataPoint,
}

// ContainerMemoryPressureMetrics is the PSIMetrics dispatch table for container.memory.pressure.{avg,time}.
var ContainerMemoryPressureMetrics = PSIMetrics{
	Avg:  (*MetricsBuilder).RecordContainerMemoryPressureAvgDataPoint,
	Time: (*MetricsBuilder).RecordContainerMemoryPressureTimeDataPoint,
}

// ContainerIOPressureMetrics is the PSIMetrics dispatch table for container.io.pressure.{avg,time}.
var ContainerIOPressureMetrics = PSIMetrics{
	Avg:  (*MetricsBuilder).RecordContainerIoPressureAvgDataPoint,
	Time: (*MetricsBuilder).RecordContainerIoPressureTimeDataPoint,
}
