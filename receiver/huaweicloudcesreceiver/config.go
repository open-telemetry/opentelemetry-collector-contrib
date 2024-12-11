// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver"

import (
	"errors"
	"fmt"
	"slices"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
)

var (
	// Predefined error responses for configuration validation failures
	errInvalidCollectionInterval = errors.New(`invalid period; must be less than "collection_interval"`)
	errMissingProjectID          = errors.New(`"project_id" is not specified in config`)
	errMissingRegionID           = errors.New(`"region_id" is not specified in config`)
	errInvalidProxy              = errors.New(`"proxy_address" must be specified if "proxy_user" or "proxy_password" is set"`)
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
	huaweiSessionConfig `mapstructure:",squash"`

	// ProjectID is a string to reference project where metrics should be associated with.
	// If ProjectID is not filled in, the SDK will automatically call the IAM service to query the project id corresponding to the region.
	ProjectID string `mapstructure:"project_id"`

	// RegionID is the ID of the CES region.
	RegionID string `mapstructure:"region_id"`

	// How retrieved data from Cloud Eye is aggregated.
	// Possible values are 1, 300, 1200, 3600, 14400, and 86400.
	// 1: Cloud Eye performs no aggregation and displays raw data.
	// 300: Cloud Eye aggregates data every 5 minutes.
	// 1200: Cloud Eye aggregates data every 20 minutes.
	// 3600: Cloud Eye aggregates data every hour.
	// 14400: Cloud Eye aggregates data every 4 hours.
	// 86400: Cloud Eye aggregates data every 24 hours.
	// For details about the aggregation, see https://support.huaweicloud.com/intl/en-us/ces_faq/ces_faq_0009.html
	Period int32 `mapstructure:"period"`

	// Data aggregation method. The supported values ​​are max, min, average, sum, variance.
	Filter string `mapstructure:"filter"`

	BackOffConfig configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

type huaweiSessionConfig struct {
	AccessKey configopaque.String `mapstructure:"access_key"`

	SecretKey configopaque.String `mapstructure:"secret_key"`
	// Number of seconds before timing out a request.
	NoVerifySSL bool `mapstructure:"no_verify_ssl"`
	// Upload segments to AWS X-Ray through a proxy.
	ProxyAddress  string `mapstructure:"proxy_address"`
	ProxyUser     string `mapstructure:"proxy_user"`
	ProxyPassword string `mapstructure:"proxy_password"`
}

var _ component.Config = (*Config)(nil)

// These valid periods are defined by CES API constraints: https://support.huaweicloud.com/intl/en-us/api-ces/ces_03_0034.html#section3
var validPeriods = []int32{1, 300, 1200, 3600, 14400, 86400}

// These valid filters are defined by CES API constraints: https://support.huaweicloud.com/intl/en-us/api-ces/ces_03_0034.html#section3
var validFilters = map[string]model.ShowMetricDataRequestFilter{
	"max":      model.GetShowMetricDataRequestFilterEnum().MAX,
	"min":      model.GetShowMetricDataRequestFilterEnum().MIN,
	"average":  model.GetShowMetricDataRequestFilterEnum().AVERAGE,
	"sum":      model.GetShowMetricDataRequestFilterEnum().SUM,
	"variance": model.GetShowMetricDataRequestFilterEnum().VARIANCE,
}

// Validate config
func (config *Config) Validate() error {
	var err error
	if config.RegionID == "" {
		err = multierr.Append(err, errMissingRegionID)
	}

	if config.ProjectID == "" {
		err = multierr.Append(err, errMissingProjectID)
	}
	if index := slices.Index(validPeriods, config.Period); index == -1 {
		err = multierr.Append(err, fmt.Errorf("invalid period: got %d; must be one of %v", config.Period, validPeriods))
	}
	if _, ok := validFilters[config.Filter]; !ok {
		var validFiltersSlice []string
		for key := range validFilters {
			validFiltersSlice = append(validFiltersSlice, key)
		}
		err = multierr.Append(err, fmt.Errorf("invalid filter: got %s; must be one of %v", config.Filter, validFiltersSlice))
	}
	if config.Period >= int32(config.CollectionInterval.Seconds()) {
		err = multierr.Append(err, errInvalidCollectionInterval)
	}

	// Validate that ProxyAddress is provided if ProxyUser or ProxyPassword is set
	if (config.ProxyUser != "" || config.ProxyPassword != "") && config.ProxyAddress == "" {
		err = multierr.Append(err, errInvalidProxy)
	}

	return err
}
