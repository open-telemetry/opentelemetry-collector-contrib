// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudspannerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	minCollectionIntervalSeconds = 60
	maxTopMetricsQueryMaxRows    = 100
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`

	TopMetricsQueryMaxRows            int       `mapstructure:"top_metrics_query_max_rows"`
	BackfillEnabled                   bool      `mapstructure:"backfill_enabled"`
	CardinalityTotalLimit             int       `mapstructure:"cardinality_total_limit"`
	Projects                          []Project `mapstructure:"projects"`
	HideTopnLockstatsRowrangestartkey bool      `mapstructure:"hide_topn_lockstats_rowrangestartkey"`
}

type Project struct {
	ID                string     `mapstructure:"project_id"`
	ServiceAccountKey string     `mapstructure:"service_account_key"`
	Instances         []Instance `mapstructure:"instances"`
}

type Instance struct {
	ID        string   `mapstructure:"instance_id"`
	Databases []string `mapstructure:"databases"`
}

func (config *Config) Validate() error {
	if config.CollectionInterval.Seconds() < minCollectionIntervalSeconds {
		return fmt.Errorf("\"collection_interval\" must be not lower than %v seconds, current value is %v seconds", minCollectionIntervalSeconds, config.CollectionInterval.Seconds())
	}

	if config.TopMetricsQueryMaxRows <= 0 {
		return fmt.Errorf("\"top_metrics_query_max_rows\" must be positive: %v", config.TopMetricsQueryMaxRows)
	}

	if config.TopMetricsQueryMaxRows > maxTopMetricsQueryMaxRows {
		return fmt.Errorf("\"top_metrics_query_max_rows\" must be not greater than %v, current value is %v", maxTopMetricsQueryMaxRows, config.TopMetricsQueryMaxRows)
	}

	if config.CardinalityTotalLimit < 0 {
		return fmt.Errorf("\"cardinality_total_limit\" must be not negative, current value is %v", config.CardinalityTotalLimit)
	}

	if len(config.Projects) == 0 {
		return errors.New("missing required field \"projects\" or its value is empty")
	}

	for _, project := range config.Projects {
		if err := project.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (project Project) Validate() error {
	if project.ID == "" {
		return errors.New(`field "project_id" is required and cannot be empty for project configuration`)
	}

	if len(project.Instances) == 0 {
		return errors.New("field \"instances\" is required and cannot be empty for project configuration")
	}

	for _, instance := range project.Instances {
		if err := instance.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (instance Instance) Validate() error {
	if instance.ID == "" {
		return errors.New("field \"instance_id\" is required and cannot be empty for instance configuration")
	}

	if len(instance.Databases) == 0 {
		return errors.New("field \"databases\" is required and cannot be empty for instance configuration")
	}

	for _, database := range instance.Databases {
		if database == "" {
			return errors.New("field \"databases\" contains empty database names")
		}
	}

	return nil
}
