// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows && !darwin

package memoryscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"

import (
	"github.com/shirou/gopsutil/v4/mem"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func (*memoryScraper) recordSystemSpecificMetrics(_ pcommon.Timestamp, _ *mem.VirtualMemoryStat) {
	// no-op for non-linux, non-windows, non-darwin platforms
}
