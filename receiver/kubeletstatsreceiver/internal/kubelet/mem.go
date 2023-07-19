// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addMemoryMetrics(mb *metadata.MetricsBuilder, memoryMetrics metadata.MemoryMetrics, s *stats.MemoryStats, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}
	for _, recordDataPoint := range memoryMetrics.Available {
		recordIntDataPoint(mb, recordDataPoint, s.AvailableBytes, currentTime)
	}
	for _, recordDataPoint := range memoryMetrics.Usage {
		recordIntDataPoint(mb, recordDataPoint, s.UsageBytes, currentTime)
	}
	for _, recordDataPoint := range memoryMetrics.Rss {
		recordIntDataPoint(mb, recordDataPoint, s.RSSBytes, currentTime)
	}
	for _, recordDataPoint := range memoryMetrics.WorkingSet {
		recordIntDataPoint(mb, recordDataPoint, s.WorkingSetBytes, currentTime)
	}
	for _, recordDataPoint := range memoryMetrics.PageFaults {
		recordIntDataPoint(mb, recordDataPoint, s.PageFaults, currentTime)
	}
	for _, recordDataPoint := range memoryMetrics.MajorPageFaults {
		recordIntDataPoint(mb, recordDataPoint, s.MajorPageFaults, currentTime)
	}
}
