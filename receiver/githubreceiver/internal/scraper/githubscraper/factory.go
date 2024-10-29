// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/scraper/githubscraper"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

// This file implements factory for the GitHub Scraper as part of the GitHub Receiver

const (
	defaultHTTPTimeout = 15 * time.Second
)

type Factory struct{}

func (f *Factory) CreateDefaultConfig() internal.Config {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = defaultHTTPTimeout
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		ClientConfig:         clientConfig,
	}
}

func (f *Factory) CreateMetricsScraper(
	ctx context.Context,
	params receiver.Settings,
	cfg internal.Config,
) (scraperhelper.Scraper, error) {
	conf := cfg.(*Config)
	s := newGitHubScraper(ctx, params, conf)

	return scraperhelper.NewScraper(
		metadata.Type,
		s.scrape,
		scraperhelper.WithStart(s.start),
	)
}
