// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package raidscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/raidscraper"

import (
	"context"
	"slices"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/raidscraper/internal/metadata"
)

// scraper for Uptime Metrics
type raidScraper struct {
	settings       scraper.Settings
	config         *Config
	mb             *metadata.MetricsBuilder
	includeDevices filterset.FilterSet
	excludeDevices filterset.FilterSet

	// for mocking
	getMdStats func() ([]MDStat, error)
	getMdraids func() ([]Mdraid, error)
}

type MDStat struct {
	// Name of the device.
	Name string
	// activity-state of the device.
	ActivityState string
	// Number of active disks.
	DisksActive int64
	// Total number of disks the device requires.
	DisksTotal int64
	// Number of failed disks.
	DisksFailed int64
	// Number of "down" disks. (the _ indicator in the status line)
	DisksDown int64
	// Spare disks in the device.
	DisksSpare int64
	// Number of blocks the device holds.
	BlocksTotal int64
	// Number of blocks on the device that are in sync.
	BlocksSynced int64
	// Number of blocks on the device that need to be synced.
	BlocksToBeSynced int64
	// progress percentage of current sync
	BlocksSyncedPct float64
	// estimated finishing time for current sync (in minutes)
	BlocksSyncedFinishTime float64
	// current sync speed (in Kilobytes/sec)
	BlocksSyncedSpeed float64
	// Name of md component devices
	Devices []string
}

type Mdraid struct {
	Device          string            // Kernel device name of array.
	Level           string            // mdraid level.
	ArrayState      string            // State of the array.
	MetadataVersion string            // mdraid metadata version.
	Disks           uint64            // Number of devices in a fully functional array.
	Components      []MdraidComponent // mdraid component devices.
	UUID            string            // UUID of the array.

	// The following item is only valid for raid0, 4, 5, 6 and 10.
	ChunkSize uint64 // Chunk size

	// The following items are only valid for raid1, 4, 5, 6 and 10.
	DegradedDisks uint64  // Number of degraded disks in the array.
	SyncAction    string  // Current sync action.
	SyncCompleted float64 // Fraction (0-1) representing the completion status of current sync operation.
}

type MdraidComponent struct {
	Device string // Kernel device name.
	State  string // Current state of device.
}

func (s *raidScraper) start(ctx context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *raidScraper) filterMdStatsByDevice(stats []MDStat) []MDStat {
	if s.includeDevices == nil && s.excludeDevices == nil {
		return stats
	}

	for i, d := range stats {
		if !s.includeDevice(d.Name) {
			stats = slices.Delete(stats, i, i+1)
		}
	}
	return stats
}

func (s *raidScraper) filterMdraidsByDevice(stats []Mdraid) []Mdraid {
	if s.includeDevices == nil && s.excludeDevices == nil {
		return stats
	}

	for i, d := range stats {
		if !s.includeDevice(d.Device) {
			stats = slices.Delete(stats, i, i+1)
		}
	}
	return stats
}

func (s *raidScraper) includeDevice(deviceName string) bool {
	return (s.includeDevices == nil || s.includeDevices.Matches(deviceName)) &&
		(s.excludeDevices == nil || !s.excludeDevices.Matches(deviceName))
}

func (s *raidScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	if s.getMdStats != nil {
		mdStats, err := s.getMdStats()
		if err != nil {
			return pmetric.NewMetrics(), err
		}

		mdStats = s.filterMdStatsByDevice(mdStats)

		for _, mdStat := range mdStats {

			stateVals := make(map[string]int64)
			stateVals[mdStat.ActivityState] = 1

			s.mb.RecordMdBlocksTotalDataPoint(now, mdStat.BlocksTotal, mdStat.Name)
			s.mb.RecordMdBlocksSyncedDataPoint(now, mdStat.BlocksSynced, mdStat.Name)
			s.mb.RecordMdDisksRequiredDataPoint(now, mdStat.DisksTotal, mdStat.Name)
			s.mb.RecordMdDisksDataPoint(now, mdStat.DisksActive, mdStat.Name, "active")
			s.mb.RecordMdDisksDataPoint(now, mdStat.DisksFailed, mdStat.Name, "failed")
			s.mb.RecordMdDisksDataPoint(now, mdStat.DisksSpare, mdStat.Name, "spare")
			s.mb.RecordMdStateDataPoint(now, stateVals["active"], mdStat.Name, "active")
			s.mb.RecordMdStateDataPoint(now, stateVals["inactive"], mdStat.Name, "inactive")
			s.mb.RecordMdStateDataPoint(now, stateVals["recovering"], mdStat.Name, "recovering")
			s.mb.RecordMdStateDataPoint(now, stateVals["resyncing"], mdStat.Name, "resync")
			s.mb.RecordMdStateDataPoint(now, stateVals["checking"], mdStat.Name, "check")
		}
	}

	if s.getMdraids != nil {
		mdRaids, err := s.getMdraids()
		if err != nil {
			return pmetric.NewMetrics(), err
		}

		mdRaids = s.filterMdraidsByDevice(mdRaids)

		for _, mdRaid := range mdRaids {
			s.mb.RecordMdRaidDisksDataPoint(now, int64(mdRaid.Disks), mdRaid.Device)
			s.mb.RecordMdRaidDegradedDataPoint(now, int64(mdRaid.DegradedDisks), mdRaid.Device)
		}
	}

	return s.mb.Emit(), nil
}
