// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

var (
	// Hardcoded supported-OS list pending mdatagen `supported_os` annotation support.
	// See https://github.com/open-telemetry/opentelemetry-collector/issues/15020;
	// migrate to a metadata.yaml declaration and drop the runtime.GOOS check once it lands.
	supportedPlatforms = []string{
		"linux",
		"windows",
		"darwin",
		"freebsd",
		"aix",
		"android",
	}

	errUnsupportedPlatform = errors.New("the process scraper is unsupported on this platform")
)

// NewFactory for Process scraper.
func NewFactory() scraper.Factory {
	return scraper.NewFactory(metadata.Type, createDefaultConfig, scraper.WithMetrics(createMetricsScraper, metadata.MetricsStability))
}

// createDefaultConfig creates the default configuration for the Scraper.
func createDefaultConfig() component.Config {
	return &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
}

// createMetricsScraper creates a resource scraper based on provided config.
func createMetricsScraper(
	_ context.Context,
	settings scraper.Settings,
	cfg component.Config,
) (scraper.Metrics, error) {
	if !slices.Contains(supportedPlatforms, runtime.GOOS) {
		return nil, fmt.Errorf("%w: %s", errUnsupportedPlatform, runtime.GOOS)
	}

	pCfg := cfg.(*Config)
	validatePlatformEnabledMetrics(pCfg, settings.Logger)

	s, err := newProcessScraper(settings, pCfg)
	if err != nil {
		return nil, err
	}

	return scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
	)
}
