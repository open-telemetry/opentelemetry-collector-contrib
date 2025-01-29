// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"

import (
	"context"

	"github.com/shirou/gopsutil/v4/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
)

// Config is the configuration of a scraper.
type Config interface {
	SetRootPath(rootPath string)
}

func NewEnvVarFactory(delegate scraper.Factory, envMap common.EnvMap) scraper.Factory {
	return scraper.NewFactory(delegate.Type(), func() component.Config {
		return delegate.CreateDefaultConfig()
	}, scraper.WithMetrics(func(ctx context.Context, settings scraper.Settings, config component.Config) (scraper.Metrics, error) {
		scrp, err := delegate.CreateMetrics(ctx, settings, config)
		if err != nil {
			return nil, err
		}
		return &envVarScraper{delegate: scrp, envMap: envMap}, nil
	}, delegate.MetricsStability()))
}

type envVarScraper struct {
	delegate scraper.Metrics
	envMap   common.EnvMap
}

func (evs *envVarScraper) Start(ctx context.Context, host component.Host) error {
	ctx = context.WithValue(ctx, common.EnvKey, evs.envMap)
	return evs.delegate.Start(ctx, host)
}

func (evs *envVarScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	ctx = context.WithValue(ctx, common.EnvKey, evs.envMap)
	return evs.delegate.ScrapeMetrics(ctx)
}

func (evs *envVarScraper) Shutdown(ctx context.Context) error {
	ctx = context.WithValue(ctx, common.EnvKey, evs.envMap)
	return evs.delegate.Shutdown(ctx)
}
