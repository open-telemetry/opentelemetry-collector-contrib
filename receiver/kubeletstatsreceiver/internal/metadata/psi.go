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

var NodeCPUPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sNodeCPUPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sNodeCPUPressureTotalDataPoint,
}

var NodeMemoryPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sNodeMemoryPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sNodeMemoryPressureTotalDataPoint,
}

var NodeIOPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sNodeIoPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sNodeIoPressureTotalDataPoint,
}

// --- Pod PSI metric vars ---

var PodCPUPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sPodCPUPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sPodCPUPressureTotalDataPoint,
}

var PodMemoryPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sPodMemoryPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sPodMemoryPressureTotalDataPoint,
}

var PodIOPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordK8sPodIoPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordK8sPodIoPressureTotalDataPoint,
}

// --- Container PSI metric vars ---

var ContainerCPUPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordContainerCPUPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordContainerCPUPressureTotalDataPoint,
}

var ContainerMemoryPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordContainerMemoryPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordContainerMemoryPressureTotalDataPoint,
}

var ContainerIOPressureMetrics = PSIMetrics{
	Avg:   (*MetricsBuilder).RecordContainerIoPressureAvgDataPoint,
	Total: (*MetricsBuilder).RecordContainerIoPressureTotalDataPoint,
}
