// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

func getOSNfsStats() (*NfsStats, error) {
	return nil, nil
}

func getOSNfsdStats() (*nfsdStats, error) {
	return nil, nil
}

func CanScrapeAll() bool {
	return false
}
