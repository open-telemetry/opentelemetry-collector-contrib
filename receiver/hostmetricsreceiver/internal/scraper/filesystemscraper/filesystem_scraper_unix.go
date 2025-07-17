// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin || freebsd || openbsd || solaris

package filesystemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper/internal/metadata"
)

const fileSystemStatesLen = 3

func (s *filesystemsScraper) recordFileSystemUsageMetric(now pcommon.Timestamp, deviceUsages []*deviceUsage) {
	for _, deviceUsage := range deviceUsages {
		s.mb.RecordSystemFilesystemUsageDataPoint(
			now, int64(deviceUsage.usage.Used),
			deviceUsage.partition.Device, getMountMode(deviceUsage.partition.Opts), deviceUsage.partition.Mountpoint,
			deviceUsage.partition.Fstype,
			metadata.AttributeStateUsed)
		s.mb.RecordSystemFilesystemUsageDataPoint(
			now, int64(deviceUsage.usage.Free),
			deviceUsage.partition.Device, getMountMode(deviceUsage.partition.Opts),
			deviceUsage.partition.Mountpoint, deviceUsage.partition.Fstype,
			metadata.AttributeStateFree)
		s.mb.RecordSystemFilesystemUsageDataPoint(
			now, int64(deviceUsage.usage.Total-deviceUsage.usage.Used-deviceUsage.usage.Free),
			deviceUsage.partition.Device, getMountMode(deviceUsage.partition.Opts),
			deviceUsage.partition.Mountpoint, deviceUsage.partition.Fstype,
			metadata.AttributeStateReserved)
		s.mb.RecordSystemFilesystemUtilizationDataPoint(
			now, deviceUsage.usage.UsedPercent/100.0,
			deviceUsage.partition.Device, getMountMode(deviceUsage.partition.Opts),
			deviceUsage.partition.Mountpoint, deviceUsage.partition.Fstype)
	}
}

const systemSpecificMetricsLen = 1

func (s *filesystemsScraper) recordSystemSpecificMetrics(now pcommon.Timestamp, deviceUsages []*deviceUsage) {
	for _, deviceUsage := range deviceUsages {
		s.mb.RecordSystemFilesystemInodesUsageDataPoint(
			now, int64(deviceUsage.usage.InodesUsed), deviceUsage.partition.Device,
			getMountMode(deviceUsage.partition.Opts), deviceUsage.partition.Mountpoint,
			deviceUsage.partition.Fstype, metadata.AttributeStateUsed)
		s.mb.RecordSystemFilesystemInodesUsageDataPoint(
			now, int64(deviceUsage.usage.InodesFree), deviceUsage.partition.Device,
			getMountMode(deviceUsage.partition.Opts), deviceUsage.partition.Mountpoint,
			deviceUsage.partition.Fstype, metadata.AttributeStateFree)
	}
}
