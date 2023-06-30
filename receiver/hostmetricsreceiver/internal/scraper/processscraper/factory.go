// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"errors"
	"runtime"

	"github.com/shirou/gopsutil/v3/common"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

// This file implements Factory for Process scraper.

const (
	// TypeStr the value of "type" key in configuration.
	TypeStr = "process"
)

// Factory is the Factory for scraper.
type Factory struct {
}

// CreateDefaultConfig creates the default configuration for the Scraper.
func (f *Factory) CreateDefaultConfig() internal.Config {
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// CreateMetricsScraper creates a resource scraper based on provided config.
func (f *Factory) CreateMetricsScraper(
	_ context.Context,
	settings receiver.CreateSettings,
	cfg internal.Config,
	envMap common.EnvMap,
) (scraperhelper.Scraper, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		return nil, errors.New("process scraper only available on Linux, Windows, or MacOS")
	}

	s, err := newProcessScraper(settings, cfg.(*Config), envMap)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraper(
		TypeStr,
		s.scrape,
		scraperhelper.WithStart(s.start),
	)
}
