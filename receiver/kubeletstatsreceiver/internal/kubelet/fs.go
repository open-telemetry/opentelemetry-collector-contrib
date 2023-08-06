// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addFilesystemMetrics(rmb *metadata.ResourceMetricsBuilder, filesystemMetrics metadata.FilesystemMetrics, s *stats.FsStats, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}

	recordIntDataPoint(rmb, filesystemMetrics.Available, s.AvailableBytes, currentTime)
	recordIntDataPoint(rmb, filesystemMetrics.Capacity, s.CapacityBytes, currentTime)
	recordIntDataPoint(rmb, filesystemMetrics.Usage, s.UsedBytes, currentTime)
}
