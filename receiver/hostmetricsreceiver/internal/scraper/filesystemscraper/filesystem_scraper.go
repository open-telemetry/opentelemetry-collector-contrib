// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filesystemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/common"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper/internal/metadata"
)

const (
	standardMetricsLen = 1
	metricsLen         = standardMetricsLen + systemSpecificMetricsLen
)

// scraper for FileSystem Metrics
type scraper struct {
	settings receiver.CreateSettings
	config   *Config
	mb       *metadata.MetricsBuilder
	fsFilter fsFilter

	// for mocking gopsutil disk.Partitions & disk.Usage
	bootTime   func(context.Context) (uint64, error)
	partitions func(context.Context, bool) ([]disk.PartitionStat, error)
	usage      func(context.Context, string) (*disk.UsageStat, error)
}

type deviceUsage struct {
	partition disk.PartitionStat
	usage     *disk.UsageStat
}

// newFileSystemScraper creates a FileSystem Scraper
func newFileSystemScraper(_ context.Context, settings receiver.CreateSettings, cfg *Config) (*scraper, error) {
	fsFilter, err := cfg.createFilter()
	if err != nil {
		return nil, err
	}

	scraper := &scraper{settings: settings, config: cfg, bootTime: host.BootTimeWithContext, partitions: disk.PartitionsWithContext, usage: disk.UsageWithContext, fsFilter: *fsFilter}
	return scraper, nil
}

func (s *scraper) start(ctx context.Context, _ component.Host) error {
	ctx = context.WithValue(ctx, common.EnvKey, s.config.EnvMap)
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	ctx = context.WithValue(ctx, common.EnvKey, s.config.EnvMap)
	now := pcommon.NewTimestampFromTime(time.Now())

	var errors scrapererror.ScrapeErrors
	partitions, err := s.partitions(ctx, s.config.IncludeVirtualFS)
	if err != nil {
		if len(partitions) == 0 {
			return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
		}
		if strings.Contains(strings.ToLower(err.Error()), "locked") {
			// Log a debug message instead of an error message if a drive is
			// locked and unavailable. For this particular case, we do not want
			// to log an error message on every poll.
			// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18236
			s.settings.Logger.Debug("failed collecting locked partitions information: %w", zap.Error(err))
		} else {
			errors.AddPartial(0, fmt.Errorf("failed collecting partitions information: %w", err))
		}
	}

	usages := make([]*deviceUsage, 0, len(partitions))
	for _, partition := range partitions {
		if !s.fsFilter.includePartition(partition) {
			continue
		}
		translatedMountpoint := translateMountpoint(s.config.RootPath, partition.Mountpoint)
		usage, usageErr := s.usage(ctx, translatedMountpoint)
		if usageErr != nil {
			errors.AddPartial(0, fmt.Errorf("failed to read usage at %s: %w", translatedMountpoint, usageErr))
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

// translateMountsRootPath translates a mountpoint from the host perspective to the chrooted perspective.
func translateMountpoint(rootPath, mountpoint string) string {
	return filepath.Join(rootPath, mountpoint)
}
