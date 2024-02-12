// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

func calculateUtilization(usedBytes, capacityBytes *uint64) float64 {
	return float64(*usedBytes) / float64(*capacityBytes) * 100
}

func addFilesystemMetrics(mb *metadata.MetricsBuilder, filesystemMetrics metadata.FilesystemMetrics, s *stats.FsStats, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}
	utilization := calculateUtilization(s.UsedBytes, s.CapacityBytes)
	recordIntDataPoint(mb, filesystemMetrics.Available, s.AvailableBytes, currentTime)
	recordIntDataPoint(mb, filesystemMetrics.Capacity, s.CapacityBytes, currentTime)
	recordIntDataPoint(mb, filesystemMetrics.Usage, s.UsedBytes, currentTime)
	recordDoubleDataPoint(mb, filesystemMetrics.Utilization, &utilization, currentTime)
}
