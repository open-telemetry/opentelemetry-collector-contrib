// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
)

var defaultEndpoint = "http://localhost:9200"

var (
	errEndpointBadScheme    = errors.New("endpoint scheme must be http or https")
	errUsernameNotSpecified = errors.New("password was specified, but not username")
	errPasswordNotSpecified = errors.New("username was specified, but not password")
	errEmptyEndpoint        = errors.New("endpoint must be specified")
)

// Config is the configuration for the elasticsearch receiver
type Config struct {
	ControllerConfig scraperhelper.ControllerConfig `mapstructure:",squash"`
	ClientConfig     confighttp.ClientConfig        `mapstructure:",squash"`
	// MetricsBuilderConfig defines which metrics/attributes to enable for the scraper
	MetricsBuilderConfig metadata.MetricsBuilderConfig `mapstructure:",squash"`
	// Nodes defines the nodes to scrape.
	// See https://www.elastic.co/guide/en/elasticsearch/reference/7.9/cluster.html#cluster-nodes for which selectors may be used here.
	// If Nodes is empty, no nodes will be scraped.
	Nodes []string `mapstructure:"nodes"`
	// SkipClusterMetrics indicates whether cluster level metrics from /_cluster/* endpoints should be scraped or not.
	SkipClusterMetrics bool `mapstructure:"skip_cluster_metrics"`
	// ClusterStatsMasterOnly indicates whether cluster stats (from /_cluster/stats) should only be scraped when
	// this receiver's endpoint is the cluster's current elected master node. This is useful when running one
	// receiver instance per node and targeting the same cluster, to avoid every instance issuing the same
	// master-coordinated, cluster-wide call every collection interval. Has no effect if SkipClusterMetrics is true.
	ClusterStatsMasterOnly bool `mapstructure:"cluster_stats_master_only"`
	// Indices defines the indices to scrape.
	// See https://www.elastic.co/guide/en/elasticsearch/reference/current/indices-stats.html#index-stats-api-path-params
	// for which names are viable.
	// If Indices is empty, no indices will be scraped.
	Indices []string `mapstructure:"indices"`
	// IndexStatsMasterOnly indicates whether index stats (from /_stats) should only be scraped when this
	// receiver's endpoint is the cluster's current elected master node. Same rationale as ClusterStatsMasterOnly.
	IndexStatsMasterOnly bool `mapstructure:"index_stats_master_only"`
	// Username is the username used when making REST calls to elasticsearch. Must be specified if Password is. Not required.
	Username string `mapstructure:"username"`
	// Password is the password used when making REST calls to elasticsearch. Must be specified if Username is. Not required.
	Password configopaque.String `mapstructure:"password"`
}

// Validate validates the given config, returning an error specifying any issues with the config.
func (cfg *Config) Validate() error {
	var combinedErr error
	if err := invalidCredentials(cfg.Username, string(cfg.Password)); err != nil {
		combinedErr = err
	}

	if cfg.ClientConfig.Endpoint == "" {
		return errors.Join(combinedErr, errEmptyEndpoint)
	}

	u, err := url.Parse(cfg.ClientConfig.Endpoint)
	if err != nil {
		return errors.Join(
			combinedErr,
			fmt.Errorf("invalid endpoint '%s': %w", cfg.ClientConfig.Endpoint, err),
		)
	}

	switch u.Scheme {
	case "http", "https": // ok
	default:
		return errors.Join(combinedErr, errEndpointBadScheme)
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
