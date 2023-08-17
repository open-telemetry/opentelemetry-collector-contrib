package kubelet

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

func addRuntimeImageFsMetrics(mb *metadata.MetricsBuilder, runtimeImageFsMetrics metadata.RuntimeImageFsMetrics, s *stats.RuntimeStats, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}
	recordIntDataPoint(mb, runtimeImageFsMetrics.Available, s.ImageFs.AvailableBytes, currentTime)
	recordIntDataPoint(mb, runtimeImageFsMetrics.Capacity, s.ImageFs.CapacityBytes, currentTime)
	recordIntDataPoint(mb, runtimeImageFsMetrics.Inodes, s.ImageFs.Inodes, currentTime)
	recordIntDataPoint(mb, runtimeImageFsMetrics.InodesUsed, s.ImageFs.InodesUsed, currentTime)
	recordIntDataPoint(mb, runtimeImageFsMetrics.InodesFree, s.ImageFs.InodesFree, currentTime)
}
