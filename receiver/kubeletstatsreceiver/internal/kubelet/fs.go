// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addFilesystemMetrics(mb *metadata.MetricsBuilder, filesystemMetrics metadata.FilesystemMetrics, s *stats.FsStats, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}
	for _, recordDataPoint := range filesystemMetrics.Available {
		recordIntDataPoint(mb, recordDataPoint, s.AvailableBytes, currentTime)
	}
	for _, recordDataPoint := range filesystemMetrics.Capacity {
		recordIntDataPoint(mb, recordDataPoint, s.CapacityBytes, currentTime)
	}
	for _, recordDataPoint := range filesystemMetrics.Usage {
		recordIntDataPoint(mb, recordDataPoint, s.UsedBytes, currentTime)
	}
}
