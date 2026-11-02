// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filesystemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper/internal/metadata"
)

const (
	standardMetricsLen = 1
	metricsLen         = standardMetricsLen + systemSpecificMetricsLen
)

// filesystemsScraper for FileSystem Metrics
type filesystemsScraper struct {
	settings scraper.Settings
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
func newFileSystemScraper(_ context.Context, settings scraper.Settings, cfg *Config) (*filesystemsScraper, error) {
	fsFilter, err := cfg.createFilter()
	if err != nil {
		return nil, err
	}

	scraper := &filesystemsScraper{settings: settings, config: cfg, bootTime: host.BootTimeWithContext, partitions: disk.PartitionsWithContext, usage: disk.UsageWithContext, fsFilter: *fsFilter}
	return scraper, nil
}

func (s *filesystemsScraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *filesystemsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
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

	type mountKey struct {
		mountpoint string
		device     string
	}
	seen := map[mountKey]struct{}{}

	for _, partition := range partitions {
		key := mountKey{
			mountpoint: partition.Mountpoint,
			device:     partition.Device,
		}
		if _, ok := seen[key]; partition.Mountpoint != "" && ok {
			continue
		}
		seen[key] = struct{}{}

		if !s.fsFilter.includePartition(partition) {
			continue
		}
		translatedMountpoint := translateMountpoint(ctx, s.config.rootPath, partition.Mountpoint)
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
	if slices.Contains(opts, "rw") {
		return "rw"
	} else if slices.Contains(opts, "ro") {
		return "ro"
	}
	return "unknown"
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
func translateMountpoint(ctx context.Context, rootPath, mountpoint string) string {
	if env, ok := ctx.Value(common.EnvKey).(common.EnvMap); ok {
		mountInfo := env[common.EnvKeyType("HOST_PROC_MOUNTINFO")]
		if mountInfo != "" {
			return mountpoint
		}
	}

	// gopsutil first reads <rootPath>/proc/1/mountinfo, and if this fails it reads <rootPath>/proc/self/mountinfo as a fallback.
	// If the collector process runs in a different mount namespace than PID 1 on the host (e.g. on Kubernetes):
	//
	// <rootPath>/proc/1/mountinfo contains the mountpoints from the perspective of PID 1 on the host (without rootPath prefix)
	// <rootPath>/proc/self/mountinfo contains the mountpoints from the perspective of the collector process (with the rootPath prefix)
	//
	// Example on Fedora 43:
	// $ docker run -v /:/hostfs:ro alpine cat /hostfs/proc/1/mountinfo | grep ext4
	// 61 73 259:2 / /boot rw,relatime shared:84 - ext4 /dev/nvme0n1p2 rw,seclabel
	//
	// $ docker run -v /:/hostfs:ro alpine cat /hostfs/proc/self/mountinfo | grep ext4
	// 5111 5048 259:2 / /hostfs/boot ro,relatime master:84 - ext4 /dev/nvme0n1p2 rw,seclabel
	//
	// Therefore, do not add rootPath if the mountpath has already a rootPath prefix.
	sep := string(filepath.Separator)
	if rootPath != "" && rootPath != sep &&
		strings.HasPrefix(filepath.Clean(mountpoint)+sep, filepath.Clean(rootPath)+sep) {
		return mountpoint
	}

	return filepath.Join(rootPath, mountpoint)
}
