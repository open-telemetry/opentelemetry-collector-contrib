// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package psiscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/psiscraper"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
)

const (
	metricsLen = 0
)

// psiScraper for PSI Metrics
type psiScraper struct {
	settings scraper.Settings
	config   *Config
}

// newPSIScraper creates a PSI scraper
func newPSIScraper(_ context.Context, settings scraper.Settings, cfg *Config) *psiScraper {
	return &psiScraper{settings: settings, config: cfg}
}

// start
func (s *psiScraper) start(_ context.Context, _ component.Host) error {
	return errors.New("PSI scraper is only available on Linux")
}

// scrape
func (s *psiScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	return pmetric.NewMetrics(), errors.New("PSI scraper is only available on Linux")
}
