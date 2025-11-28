// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/scraper/githubscraper"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

// Config relating to GitHub Metric Scraper.
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	confighttp.ClientConfig       `mapstructure:",squash"`
	internal.ScraperConfig
	// GitHubOrg is the name of the GitHub organization to scrape (github scraper only)
	GitHubOrg string `mapstructure:"github_org"`
	// SearchQuery is the query to use when defining a custom search for repository data
	SearchQuery string `mapstructure:"search_query"`
	// ConcurrencyLimit limits the number of concurrent repository processing goroutines
	// Default is 50 to stay well under GitHub's 100 concurrent request limit
	ConcurrencyLimit int `mapstructure:"concurrency_limit"`
	// MergedPRLookbackDays limits how far back to look for merged pull requests
	// Default is 30 days. Set to 0 to fetch all merged PRs (backwards compatible)
	MergedPRLookbackDays int `mapstructure:"merged_pr_lookback_days"`
}

// Validate validates the configuration
func (cfg *Config) Validate() error {
	if cfg.ConcurrencyLimit < 0 {
		return errors.New("concurrency_limit must be non-negative")
	}
	return nil
}
