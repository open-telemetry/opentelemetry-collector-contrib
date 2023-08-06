// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addMemoryMetrics(rmb *metadata.ResourceMetricsBuilder, memoryMetrics metadata.MemoryMetrics, s *stats.MemoryStats, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}

	recordIntDataPoint(rmb, memoryMetrics.Available, s.AvailableBytes, currentTime)
	recordIntDataPoint(rmb, memoryMetrics.Usage, s.UsageBytes, currentTime)
	recordIntDataPoint(rmb, memoryMetrics.Rss, s.RSSBytes, currentTime)
	recordIntDataPoint(rmb, memoryMetrics.WorkingSet, s.WorkingSetBytes, currentTime)
	recordIntDataPoint(rmb, memoryMetrics.PageFaults, s.PageFaults, currentTime)
	recordIntDataPoint(rmb, memoryMetrics.MajorPageFaults, s.MajorPageFaults, currentTime)
}
