// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows
// +build !linux,!windows

package diskscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"

import (
	"github.com/shirou/gopsutil/v3/disk"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const systemSpecificMetricsLen = 0

func (s *scraper) recordSystemSpecificDataPoints(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
}
