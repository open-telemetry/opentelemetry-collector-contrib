// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin && !freebsd && !openbsd && !solaris
// +build !linux,!darwin,!freebsd,!openbsd,!solaris

package filesystemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper/internal/metadata"
)

const fileSystemStatesLen = 2

func (s *scraper) recordFileSystemUsageMetric(now pcommon.Timestamp, deviceUsages []*deviceUsage) {
	for _, deviceUsage := range deviceUsages {
		s.mb.RecordSystemFilesystemUsageDataPoint(
			now, int64(deviceUsage.usage.Used),
			deviceUsage.partition.Device, getMountMode(deviceUsage.partition.Opts),
			deviceUsage.partition.Mountpoint, deviceUsage.partition.Fstype,
			metadata.AttributeStateUsed)
		s.mb.RecordSystemFilesystemUsageDataPoint(
			now, int64(deviceUsage.usage.Free),
			deviceUsage.partition.Device, getMountMode(deviceUsage.partition.Opts),
			deviceUsage.partition.Mountpoint, deviceUsage.partition.Fstype,
			metadata.AttributeStateFree)
		s.mb.RecordSystemFilesystemUtilizationDataPoint(
			now, deviceUsage.usage.UsedPercent/100.0,
			deviceUsage.partition.Device, getMountMode(deviceUsage.partition.Opts),
			deviceUsage.partition.Mountpoint, deviceUsage.partition.Fstype)
	}
}

const systemSpecificMetricsLen = 0

func (s *scraper) recordSystemSpecificMetrics(now pcommon.Timestamp, deviceUsages []*deviceUsage) {
}
