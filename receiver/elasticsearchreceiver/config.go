// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
)

var (
	defaultEndpoint = "http://localhost:9200"
)

var (
	errEndpointBadScheme    = errors.New("endpoint scheme must be http or https")
	errUsernameNotSpecified = errors.New("password was specified, but not username")
	errPasswordNotSpecified = errors.New("username was specified, but not password")
	errEmptyEndpoint        = errors.New("endpoint must be specified")
)

// Config is the configuration for the elasticsearch receiver
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	// MetricsBuilderConfig defines which metrics/attributes to enable for the scraper
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	// Nodes defines the nodes to scrape.
	// See https://www.elastic.co/guide/en/elasticsearch/reference/7.9/cluster.html#cluster-nodes for which selectors may be used here.
	// If Nodes is empty, no nodes will be scraped.
	Nodes []string `mapstructure:"nodes"`
	// SkipClusterMetrics indicates whether cluster level metrics from /_cluster/* endpoints should be scraped or not.
	SkipClusterMetrics bool `mapstructure:"skip_cluster_metrics"`
	// Indices defines the indices to scrape.
	// See https://www.elastic.co/guide/en/elasticsearch/reference/current/indices-stats.html#index-stats-api-path-params
	// for which names are viable.
	// If Indices is empty, no indices will be scraped.
	Indices []string `mapstructure:"indices"`
	// Username is the username used when making REST calls to elasticsearch. Must be specified if Password is. Not required.
	Username string `mapstructure:"username"`
	// Password is the password used when making REST calls to elasticsearch. Must be specified if Username is. Not required.
	Password configopaque.String `mapstructure:"password"`
}

// Validate validates the given config, returning an error specifying any issues with the config.
func (cfg *Config) Validate() error {
	var combinedErr error
	if err := invalidCredentials(cfg.Username, string(cfg.Password)); err != nil {
		combinedErr = multierr.Append(combinedErr, err)
	}

	if cfg.Endpoint == "" {
		return multierr.Append(combinedErr, errEmptyEndpoint)
	}

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return multierr.Append(
			combinedErr,
			fmt.Errorf("invalid endpoint '%s': %w", cfg.Endpoint, err),
		)
	}

	switch u.Scheme {
	case "http", "https": // ok
	default:
		return multierr.Append(combinedErr, errEndpointBadScheme)
	}

	return combinedErr
}

// invalidCredentials returns true if only one username or password is not empty.
func invalidCredentials(username, password string) error {
	if username == "" && password != "" {
		return errUsernameNotSpecified
	}

	if password == "" && username != "" {
		return errPasswordNotSpecified
	}
	return nil
}
