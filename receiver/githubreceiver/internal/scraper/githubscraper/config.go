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
	confighttp.ClientConfig `mapstructure:",squash"`
	internal.ScraperConfig
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	// ConcurrencyLimit limits the number of goroutines spawned by repository
	// Default is 50
	// Set to 0 for unlimited concurrency (not recommended)
	ConcurrencyLimit int `mapstructure:"concurrency_limit"`
	// GitHubOrg is the name of the GitHub organization to scrape (github scraper only)
	GitHubOrg string `mapstructure:"github_org"`
	// MergedPRLookbackDays limits how far back to query for merged pull requests in days.
	// Default is 30 days. Set to 0 to fetch all merged PRs (no time filtering).
	// Open PRs are always fetched regardless of this setting.
	MergedPRLookbackDays int `mapstructure:"merged_pr_lookback_days"`
	// SearchQuery is the query to use when defining a custom search for repository data
	SearchQuery string `mapstructure:"search_query"`
}

// Validate validates the configuration
func (cfg *Config) Validate() error {
	if cfg.ConcurrencyLimit < 0 {
		return errors.New("concurrency_limit must be non-negative")
	}
	if cfg.MergedPRLookbackDays < 0 {
		return errors.New("merged_pr_lookback_days must be non-negative")
	}
	return nil
}
