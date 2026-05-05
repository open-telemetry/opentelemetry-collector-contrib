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

	recordIntDataPoint(mb, filesystemMetrics.Available, s.AvailableBytes, currentTime)
	recordIntDataPoint(mb, filesystemMetrics.Capacity, s.CapacityBytes, currentTime)
	recordIntDataPoint(mb, filesystemMetrics.Usage, s.UsedBytes, currentTime)
}

func addEphemeralStorageMetrics(mb *metadata.MetricsBuilder, esMetrics metadata.EphemeralStorageMetrics, s *stats.FsStats, fsType metadata.AttributeFsType, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}

	recordIntDataPointWithFsType(mb, esMetrics.Usage, s.UsedBytes, fsType, currentTime)
}

func recordIntDataPointWithFsType(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordIntDataPointWithFsTypeFunc, value *uint64, fsType metadata.AttributeFsType, currentTime pcommon.Timestamp) {
	if value == nil || recordDataPoint == nil {
		return
	}
	recordDataPoint(mb, currentTime, int64(*value), fsType)
}
