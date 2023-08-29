// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows && !linux
// +build !windows,!linux

package pagingscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"

import "github.com/shirou/gopsutil/v3/mem"

func getPageFileStats() ([]*pageFileStats, error) {
	vmem, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	return []*pageFileStats{{
		deviceName:  "", // We do not support per-device swap
		usedBytes:   vmem.SwapTotal - vmem.SwapFree - vmem.SwapCached,
		freeBytes:   vmem.SwapFree,
		cachedBytes: &vmem.SwapCached,
	}}, nil
}
