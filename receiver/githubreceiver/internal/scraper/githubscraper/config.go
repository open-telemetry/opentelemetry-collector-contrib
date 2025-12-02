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
	// Set to 0 for unlimited concurrency (not recommended for >100 repos)
	ConcurrencyLimit int `mapstructure:"concurrency_limit"`
}

// Validate validates the configuration
func (cfg *Config) Validate() error {
	if cfg.ConcurrencyLimit < 0 {
		return errors.New("concurrency_limit must be non-negative")
	}
	return nil
}
