// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitmetricsreceiver/internal/scraper/githubscraper"

import (
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitmetricsreceiver/internal/metadata"
)

// Config relating to Github Metric Scraper.
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	internal.ScraperConfig
	// GitHubOrg is the name of the GitHub organization to srape (github scraper only)
	GitHubOrg string `mapstructure:"github_org"`
}
