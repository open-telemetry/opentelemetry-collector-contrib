// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Driver                                  string  `mapstructure:"driver"`
	DataSource                              string  `mapstructure:"datasource"`
	Queries                                 []Query `mapstructure:"queries"`
}

type Query struct {
	SQL     string   `mapstructure:"sql"`
	Metrics []Metric `mapstructure:"metrics"`
}

type Metric struct {
	MetricName       string   `mapstructure:"metric_name"`
	ValueColumn      string   `mapstructure:"value_column"`
	AttributeColumns []string `mapstructure:"attribute_columns"`
	IsMonotonic      bool     `mapstructure:"is_monotonic"`
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
		},
	}
}
