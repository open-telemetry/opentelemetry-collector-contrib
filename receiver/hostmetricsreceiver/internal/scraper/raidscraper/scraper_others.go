// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package raidscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/raidscraper"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"go.opentelemetry.io/collector/scraper"
)

// newRaidScraper creates raid related metrics
func newRaidScraper(_ context.Context, settings scraper.Settings, cfg *Config) (*raidScraper, error) {

	scraper := &raidScraper{settings: settings, config: cfg}
	var err error

	if len(cfg.Include.Devices) > 0 {
		scraper.includeDevices, err = filterset.CreateFilterSet(cfg.Include.Devices, &cfg.Include.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating device include filters: %w", err)
		}
	}

	if len(cfg.Exclude.Devices) > 0 {
		scraper.excludeDevices, err = filterset.CreateFilterSet(cfg.Exclude.Devices, &cfg.Exclude.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating device exclude filters: %w", err)
		}
	}

	return scraper, nil
}
