// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pagingscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"

type pageFileStats struct {
	deviceName  string // Optional
	usedBytes   uint64
	freeBytes   uint64
	totalBytes  uint64
	cachedBytes *uint64 // Optional
}
