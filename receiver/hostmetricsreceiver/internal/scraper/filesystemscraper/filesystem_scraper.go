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

package filesystemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper/internal/metadata"
)

const (
	standardMetricsLen = 1
	metricsLen         = standardMetricsLen + systemSpecificMetricsLen
)

// scraper for FileSystem Metrics
type scraper struct {
	config   *Config
	mb       *metadata.MetricsBuilder
	fsFilter fsFilter

	// for mocking gopsutil disk.Partitions & disk.Usage
	bootTime   func() (uint64, error)
	partitions func(bool) ([]disk.PartitionStat, error)
	usage      func(string) (*disk.UsageStat, error)
}

type deviceUsage struct {
	partition disk.PartitionStat
	usage     *disk.UsageStat
}

// newFileSystemScraper creates a FileSystem Scraper
func newFileSystemScraper(_ context.Context, cfg *Config) (*scraper, error) {
	fsFilter, err := cfg.createFilter()
	if err != nil {
		return nil, err
	}

	scraper := &scraper{config: cfg, bootTime: host.BootTime, partitions: disk.Partitions, usage: disk.Usage, fsFilter: *fsFilter}
	return scraper, nil
}

func (s *scraper) start(context.Context, component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.Metrics, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	// omit logical (virtual) filesystems (not relevant for windows)
	partitions, err := s.partitions( /*all=*/ false)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	var errors scrapererror.ScrapeErrors
	usages := make([]*deviceUsage, 0, len(partitions))
	for _, partition := range partitions {
		if !s.fsFilter.includePartition(partition) {
			continue
		}
		usage, usageErr := s.usage(partition.Mountpoint)
		if usageErr != nil {
			errors.AddPartial(0, fmt.Errorf("failed to read usage at %s: %w", partition.Mountpoint, usageErr))
			continue
		}

		usages = append(usages, &deviceUsage{partition, usage})
	}

	if len(usages) > 0 {
		s.recordFileSystemUsageMetric(now, usages)
		s.recordSystemSpecificMetrics(now, usages)
	}

	err = errors.Combine()
	if err != nil && len(usages) == 0 {
		err = scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	return s.mb.Emit(), err
}

func getMountMode(opts []string) string {
	if exists(opts, "rw") {
		return "rw"
	} else if exists(opts, "ro") {
		return "ro"
	}
	return "unknown"
}

func exists(options []string, opt string) bool {
	for _, o := range options {
		if o == opt {
			return true
		}
	}
	return false
}

func (f *fsFilter) includePartition(partition disk.PartitionStat) bool {
	// If filters do not exist, return early.
	if !f.filtersExist || (f.includeDevice(partition.Device) &&
		f.includeFSType(partition.Fstype) &&
		f.includeMountPoint(partition.Mountpoint)) {
		return true
	}
	return false
}

func (f *fsFilter) includeDevice(deviceName string) bool {
	return (f.includeDeviceFilter == nil || f.includeDeviceFilter.Matches(deviceName)) &&
		(f.excludeDeviceFilter == nil || !f.excludeDeviceFilter.Matches(deviceName))
}

func (f *fsFilter) includeFSType(fsType string) bool {
	return (f.includeFSTypeFilter == nil || f.includeFSTypeFilter.Matches(fsType)) &&
		(f.excludeFSTypeFilter == nil || !f.excludeFSTypeFilter.Matches(fsType))
}

func (f *fsFilter) includeMountPoint(mountPoint string) bool {
	return (f.includeMountPointFilter == nil || f.includeMountPointFilter.Matches(mountPoint)) &&
		(f.excludeMountPointFilter == nil || !f.excludeMountPointFilter.Matches(mountPoint))
}
