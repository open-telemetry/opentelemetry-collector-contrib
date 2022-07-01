// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
