// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addMemoryMetrics(mb *metadata.MetricsBuilder, memoryMetrics metadata.MemoryMetrics, s *stats.MemoryStats, currentTime pcommon.Timestamp, r resources) {
	if s == nil {
		return
	}

	recordIntDataPoint(mb, memoryMetrics.Available, s.AvailableBytes, currentTime)
	recordIntDataPoint(mb, memoryMetrics.Usage, s.UsageBytes, currentTime)
	recordIntDataPoint(mb, memoryMetrics.Rss, s.RSSBytes, currentTime)
	recordIntDataPoint(mb, memoryMetrics.WorkingSet, s.WorkingSetBytes, currentTime)
	recordIntDataPoint(mb, memoryMetrics.PageFaults, s.PageFaults, currentTime)
	recordIntDataPoint(mb, memoryMetrics.MajorPageFaults, s.MajorPageFaults, currentTime)

	if s.UsageBytes != nil {
		if r.memoryLimit > 0 {
			memoryMetrics.LimitUtilization(mb, currentTime, float64(*s.UsageBytes)/float64(r.memoryLimit))
		}
		if r.memoryRequest > 0 {
			memoryMetrics.RequestUtilization(mb, currentTime, float64(*s.UsageBytes)/float64(r.memoryRequest))
		}
	}
}
