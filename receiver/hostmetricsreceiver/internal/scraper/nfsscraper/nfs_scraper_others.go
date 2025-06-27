// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

func (s *nfsScraper) getOSNfsStats() (*NfsStats, error) {
	return nil, nil
}

func (s *nfsScraper) getOSNfsdStats() (*NfsdStats, error) {
	return nil, nil
}
