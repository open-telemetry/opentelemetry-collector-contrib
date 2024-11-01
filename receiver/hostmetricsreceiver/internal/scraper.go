// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"

import (
	"context"

	"github.com/shirou/gopsutil/v4/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
)

// ScraperFactory can create a MetricScraper.
type ScraperFactory interface {
	// CreateDefaultConfig creates the default configuration for the Scraper.
	CreateDefaultConfig() Config

	// CreateMetricsScraper creates a scraper based on this config.
	// If the config is not valid, error will be returned instead.
	CreateMetricsScraper(ctx context.Context, settings receiver.Settings, cfg Config) (scraper.Metrics, error)
}

// Config is the configuration of a scraper.
type Config interface {
	SetRootPath(rootPath string)
}

type ScraperConfig struct {
	RootPath string `mapstructure:"-"`
}

func (p *ScraperConfig) SetRootPath(rootPath string) {
	p.RootPath = rootPath
}

type EnvVarScraper struct {
	delegate scraper.Metrics
	envMap   common.EnvMap
}

func NewEnvVarScraper(delegate scraper.Metrics, envMap common.EnvMap) scraper.Metrics {
	return &EnvVarScraper{delegate: delegate, envMap: envMap}
}

func (evs *EnvVarScraper) Start(ctx context.Context, host component.Host) error {
	ctx = context.WithValue(ctx, common.EnvKey, evs.envMap)
	return evs.delegate.Start(ctx, host)
}

func (evs *EnvVarScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	ctx = context.WithValue(ctx, common.EnvKey, evs.envMap)
	return evs.delegate.ScrapeMetrics(ctx)
}

func (evs *EnvVarScraper) Shutdown(ctx context.Context) error {
	ctx = context.WithValue(ctx, common.EnvKey, evs.envMap)
	return evs.delegate.Shutdown(ctx)
}
