// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver"

import (
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	maxDimensions      = 4
	maxMetricRetention = 604800
)

var (
	namespaceRegex             = regexp.MustCompile(`[A-Za-z][a-zA-Z_0-9]{0,29}\.[a-zA-Z_0-9]{1,30}`)
	namespaceForbiddenServices = map[string]any{
		"SYS": struct{}{},
		"AGT": struct{}{},
		"SRE": struct{}{},
	}
	forbiddenNamespace = "SERVICE.BMS"
)

// Config represent a configuration for the CloudWatch logs exporter.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	confighttp.ClientConfig        `mapstructure:",squash"`
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
	// Set of attributes used to configure huawei's CES SDK connection
	HuaweiSessionConfig `mapstructure:",squash"`

	// ProjectId is a string to reference project where metrics should be associated with.
	// If ProjectId is not filled in, the SDK will automatically call the IAM service to query the project id corresponding to the region.
	ProjectId string `mapstructure:"project_id"`

}

type HuaweiSessionConfig struct {
	// RegionName is the full name of the CES region exporter should send metrics to
	RegionName string `mapstructure:"region_name"`
	// Number of seconds before timing out a request.
	NoVerifySSL bool `mapstructure:"no_verify_ssl"`
	// Upload segments to AWS X-Ray through a proxy.
	ProxyAddress  string `mapstructure:"proxy_address"`
	ProxyUser     string `mapstructure:"proxy_user"`
	ProxyPassword string `mapstructure:"proxy_password"`
}

var _ component.Config = (*Config)(nil)

// Validate config
func (config *Config) Validate() error {
	// TODO validate receiver config
	return nil
}

func KeysString(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return "[" + strings.Join(keys, ", ") + "]"
}
