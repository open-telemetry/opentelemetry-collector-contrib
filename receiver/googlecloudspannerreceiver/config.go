// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	TopMetricsQueryMaxRows      int       `mapstructure:"top_metrics_query_max_rows"`
	BackfillEnabled             bool      `mapstructure:"backfill_enabled"`
	CardinalityTotalLimit       int       `mapstructure:"cardinality_total_limit"`
	Projects                    []Project `mapstructure:"projects"`
	HideTopnQuerystatsQuerytext bool      `mapstructure:"hide_topn_querystats_querytext"`
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
		return fmt.Errorf("%v %q must be not lower than %v seconds, current value is %v seconds", config.ID(),
			"collection_interval", minCollectionIntervalSeconds, config.CollectionInterval.Seconds())
	}

	if config.TopMetricsQueryMaxRows <= 0 {
		return fmt.Errorf("%v %q must be positive: %v", config.ID(), "top_metrics_query_max_rows",
			config.TopMetricsQueryMaxRows)
	}

	if config.TopMetricsQueryMaxRows > maxTopMetricsQueryMaxRows {
		return fmt.Errorf("%v %q must be not greater than %v, current value is %v", config.ID(),
			"top_metrics_query_max_rows", maxTopMetricsQueryMaxRows, config.TopMetricsQueryMaxRows)
	}

	if config.CardinalityTotalLimit < 0 {
		return fmt.Errorf("%v %q must be not negative, current value is %v",
			config.ID(), "cardinality_total_limit", config.CardinalityTotalLimit)
	}

	if len(config.Projects) <= 0 {
		return fmt.Errorf("%v missing required field %q or its value is empty", config.ID(), "projects")
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

	if len(project.Instances) <= 0 {
		return fmt.Errorf("field %q is required and cannot be empty for project configuration", "instances")
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
		return fmt.Errorf("field %q is required and cannot be empty for instance configuration", "instance_id")
	}

	if len(instance.Databases) <= 0 {
		return fmt.Errorf("field %q is required and cannot be empty for instance configuration", "databases")
	}

	for _, database := range instance.Databases {
		if database == "" {
			return fmt.Errorf("field %q contains empty database names", "databases")
		}
	}

	return nil
}
