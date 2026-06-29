// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	conntrackMetricsLen = 2
	bandwidthMetricsLen = 1
)

var allTCPStates = []string{
	"CLOSE_WAIT",
	"CLOSE",
	"CLOSING",
	"DELETE",
	"ESTABLISHED",
	"FIN_WAIT_1",
	"FIN_WAIT_2",
	"LAST_ACK",
	"LISTEN",
	"SYN_SENT",
	"SYN_RECV",
	"TIME_WAIT",
}

func (s *networkScraper) recordNetworkConntrackMetrics(ctx context.Context) error {
	if !s.config.Metrics.SystemNetworkConntrackCount.Enabled && !s.config.Metrics.SystemNetworkConntrackMax.Enabled {
		return nil
	}
	now := pcommon.NewTimestampFromTime(time.Now())
	conntrack, err := s.conntrack(ctx)
	if err != nil {
		return fmt.Errorf("failed to read conntrack info: %w", err)
	}
	s.mb.RecordSystemNetworkConntrackCountDataPoint(now, conntrack[0].ConnTrackCount)
	s.mb.RecordSystemNetworkConntrackMaxDataPoint(now, conntrack[0].ConnTrackMax)
	return nil
}

func (s *networkScraper) recordNetworkBandwidthMetrics(ctx context.Context) error {
	if !s.config.Metrics.SystemNetworkBandwidthLimit.Enabled {
		return nil
	}
	ioCounters, err := s.ioCounters(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to read network interfaces: %w", err)
	}
	ioCounters = s.filterByInterface(ioCounters)
	now := pcommon.NewTimestampFromTime(time.Now())
	for _, ioc := range ioCounters {
		speedMbps, ok := readInterfaceSpeedMbps(ioc.Name)
		if !ok {
			continue
		}
		// speed is reported in Mbit/s; convert to bytes/s.
		s.mb.RecordSystemNetworkBandwidthLimitDataPoint(now, speedMbps*125000, ioc.Name)
	}
	return nil
}

// readInterfaceSpeedMbps returns the link speed in Mbit/s from sysfs, or false
// for interfaces that report no usable speed (virtual or down links read -1).
// HOST_SYS is honored to match gopsutil under a configured root_path.
func readInterfaceSpeedMbps(device string) (int64, bool) {
	hostSys := os.Getenv("HOST_SYS")
	if hostSys == "" {
		hostSys = "/sys"
	}
	data, err := os.ReadFile(filepath.Join(hostSys, "class", "net", device, "speed"))
	if err != nil {
		return 0, false
	}
	speed, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil || speed <= 0 {
		return 0, false
	}
	return speed, true
}
