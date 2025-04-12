// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows

package diskscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"

import (
	"github.com/shirou/gopsutil/v4/disk"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const systemSpecificMetricsLen = 0

func (s *diskScraper) recordSystemSpecificDataPoints(_ pcommon.Timestamp, _ map[string]disk.IOCountersStat) {
}
