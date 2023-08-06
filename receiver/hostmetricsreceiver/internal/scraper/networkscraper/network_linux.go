// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/common"
	"go.opentelemetry.io/collector/pdata/pcommon"
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

func (s *scraper) recordNetworkConntrackMetrics() error {
	if !s.config.MetricsBuilderConfig.Metrics.SystemNetworkConntrackCount.Enabled && !s.config.MetricsBuilderConfig.Metrics.SystemNetworkConntrackMax.Enabled {
		return nil
	}
	ctx := context.WithValue(context.Background(), common.EnvKey, s.config.EnvMap)
	now := pcommon.NewTimestampFromTime(time.Now())
	conntrack, err := s.conntrack(ctx)
	if err != nil {
		return fmt.Errorf("failed to read conntrack info: %w", err)
	}
	rmb := s.mb.ResourceMetricsBuilder(pcommon.NewResource())
	rmb.RecordSystemNetworkConntrackCountDataPoint(now, conntrack[0].ConnTrackCount)
	rmb.RecordSystemNetworkConntrackMaxDataPoint(now, conntrack[0].ConnTrackMax)
	return nil
}
