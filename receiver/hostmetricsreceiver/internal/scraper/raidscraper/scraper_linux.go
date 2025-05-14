// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package raidscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/raidscraper"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"

	"github.com/prometheus/procfs"
	"github.com/prometheus/procfs/sysfs"
)

// newRaidScraper creates raid related metrics
func newRaidScraper(_ context.Context, settings scraper.Settings, cfg *Config) (*raidScraper, error) {
	scraper := &raidScraper{settings: settings, config: cfg, getMdStats: getMDStats, getMdraids: getMdraids}
	var err error

	if len(cfg.Include.Devices) > 0 {
		scraper.includeDevices, err = filterset.CreateFilterSet(cfg.Include.Devices, &cfg.Include.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating device include filters: %w", err)
		}
	}

	if len(cfg.Exclude.Devices) > 0 {
		scraper.excludeDevices, err = filterset.CreateFilterSet(cfg.Exclude.Devices, &cfg.Exclude.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating device exclude filters: %w", err)
		}
	}

	return scraper, nil
}

func getMDStats() ([]MDStat, error) {
	procFS, err := procfs.NewFS(procfs.DefaultMountPoint)
	if err != nil {
		return []MDStat{}, fmt.Errorf("failed to open procfs: %w", err)
	}
	mdstats, err := procFS.MDStat()
	if err != nil {
		return []MDStat{}, fmt.Errorf("failed to get mdstats: %w", err)
	}

	return convertMdStats(mdstats), nil
}

func getMdraids() ([]Mdraid, error) {
	sysFS, err := sysfs.NewFS("/sys")
	if err != nil {
		return []Mdraid{}, fmt.Errorf("failed to open sysfs: %w", err)
	}
	mdraids, err := sysFS.Mdraids()
	if err != nil {
		return []Mdraid{}, fmt.Errorf("failed to get mdraids: %w", err)
	}

	return convertMdraids(mdraids), nil
}

func convertMdStats(stats1 []procfs.MDStat) []MDStat {
	stats2 := []MDStat{}
	for _, stat := range stats1 {
		stats2 = append(stats2, MDStat(stat))
	}
	return stats2
}

func convertMdraids(stats1 []sysfs.Mdraid) []Mdraid {
	stats2 := []Mdraid{}
	for _, stat := range stats1 {
		stats2 = append(stats2, Mdraid{
			Device:          stat.Device,
			Level:           stat.Level,
			ArrayState:      stat.ArrayState,
			ChunkSize:       stat.ChunkSize,
			SyncAction:      stat.SyncAction,
			SyncCompleted:   stat.SyncCompleted,
			Disks:           stat.Disks,
			Components:      convertMdraidComponents(stat.Components),
			DegradedDisks:   stat.DegradedDisks,
			UUID:            stat.UUID,
			MetadataVersion: stat.MetadataVersion,
		})
	}
	return stats2
}

func convertMdraidComponents(stats1 []sysfs.MdraidComponent) []MdraidComponent {
	stats2 := []MdraidComponent{}
	for _, stat := range stats1 {
		stats2 = append(stats2, MdraidComponent(stat))
	}
	return stats2
}
