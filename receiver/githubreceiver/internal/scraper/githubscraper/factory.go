// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/scraper/githubscraper"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

// This file implements factory for the GitHub Scraper as part of the GitHub Receiver

const (
	TypeStr                     = "scraper"
	defaultConcurrencyLimit     = 50
	defaultHTTPTimeout          = 15 * time.Second
	defaultMergedPRLookbackDays = 30
)

type Factory struct{}

func (*Factory) CreateDefaultConfig() internal.Config {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = defaultHTTPTimeout
	return &Config{
		ClientConfig:         clientConfig,
		ConcurrencyLimit:     defaultConcurrencyLimit, // Default to 50 concurrent goroutines
		MergedPRLookbackDays: defaultMergedPRLookbackDays,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func (*Factory) CreateMetricsScraper(
	_ context.Context,
	params receiver.Settings,
	cfg internal.Config,
) (scraper.Metrics, error) {
	conf := cfg.(*Config)
	s := newGitHubScraper(params, conf)

	return scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
	)
}
