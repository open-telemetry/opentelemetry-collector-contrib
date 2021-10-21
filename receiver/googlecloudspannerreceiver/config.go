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

package googlecloudspannerreceiver

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`

	TopMetricsQueryMaxRows int       `mapstructure:"top_metrics_query_max_rows"`
	BackfillEnabled        bool      `mapstructure:"backfill_enabled"`
	Projects               []Project `mapstructure:"projects"`
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
	if config.CollectionInterval.Seconds() < 60 {
		return fmt.Errorf("%v `collection_interval` must be not lower than 60s, current value is %vs",
			config.ID(), config.CollectionInterval.Seconds())
	}

	if config.TopMetricsQueryMaxRows <= 0 {
		return fmt.Errorf("%v `top_metrics_query_max_rows` must be positive: %v", config.ID(), config.TopMetricsQueryMaxRows)
	}

	if config.TopMetricsQueryMaxRows > 100 {
		return fmt.Errorf("%v `top_metrics_query_max_rows` must be not greater than 100, current value is %v",
			config.ID(), config.TopMetricsQueryMaxRows)
	}

	if len(config.Projects) <= 0 {
		return fmt.Errorf("%v missing required field %v or its value is empty", config.ID(), "`projects`")
	}

	for _, project := range config.Projects {
		if err := project.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (project Project) Validate() error {
	var missingFields []string

	if project.ID == "" {
		missingFields = append(missingFields, "`project_id`")
	}

	if project.ServiceAccountKey == "" {
		missingFields = append(missingFields, "`service_account_key`")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("field(s) %v is(are) required for project configuration", strings.Join(missingFields, ", "))
	}

	if len(project.Instances) <= 0 {
		return fmt.Errorf("field %v is required and cannot be empty for project configuration", "`instances`")
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
		return fmt.Errorf("field %v is required and cannot be empty for instance configuration", "`instance_id`")
	}

	if len(instance.Databases) <= 0 {
		return fmt.Errorf("field %v is required and cannot be empty for instance configuration", "`databases`")
	}

	for _, database := range instance.Databases {
		if database == "" {
			return fmt.Errorf("field %v contains empty database names", "`databases`")
		}
	}

	return nil
}
