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

	// RetryOnFailure configures reactive retry behavior when API calls fail.
	// Retries occur on: HTTP 403 (rate limit), 429 (secondary limit), 5xx errors.
	// Uses exponential backoff from github.com/cenkalti/backoff/v5.
	RetryOnFailure RetryConfig `mapstructure:"retry_on_failure"`

	// ProactiveRetry configures proactive rate limit management.
	// When enabled, monitors GitHub API rate limit metadata and waits
	// before hitting primary rate limits.
	ProactiveRetry ProactiveRetryConfig `mapstructure:"proactive_retry"`
}

// RetryConfig controls reactive retry behavior on API failures.
type RetryConfig struct {
	// Enabled enables exponential backoff retries on API errors.
	// Default: true
	Enabled bool `mapstructure:"enabled"`
}

// ProactiveRetryConfig controls proactive rate limit management.
type ProactiveRetryConfig struct {
	// Enabled enables proactive waiting based on rate limit metadata.
	// When remaining points drop below threshold, waits until rate limit resets.
	// Default: true
	Enabled bool `mapstructure:"enabled"`

	// Threshold is the minimum remaining points before proactive waiting triggers.
	// When remaining < threshold, the scraper waits until resetAt time.
	// Default: 100 (enough for ~25 repositories at 4 points each)
	Threshold int `mapstructure:"threshold"`
}

// Validate validates the configuration
func (cfg *Config) Validate() error {
	if cfg.ConcurrencyLimit < 0 {
		return errors.New("concurrency_limit must be non-negative")
	}
	if cfg.MergedPRLookbackDays < 0 {
		return errors.New("merged_pr_lookback_days must be non-negative")
	}
	if cfg.ProactiveRetry.Threshold < 0 {
		return errors.New("proactive_retry.threshold must be non-negative")
	}
	return nil
}
