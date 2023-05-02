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

package databricksreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/databricksreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/databricksreceiver/internal/metadata"
)

func createDefaultConfig() component.Config {
	scs := scraperhelper.NewDefaultScraperControllerSettings(typeStr)
	// set the default collection interval to 30 seconds which is half of the
	// lowest job frequency of 1 minute
	scs.CollectionInterval = time.Second * 30
	return &Config{
		ScraperControllerSettings: scs,
		Metrics:                   metadata.DefaultMetricsSettings(),
	}
}

type Config struct {
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	InstanceName                            string `mapstructure:"instance_name"`
	Token                                   string `mapstructure:"token"`
	SparkOrgID                              string `mapstructure:"spark_org_id"`
	SparkEndpoint                           string `mapstructure:"spark_endpoint"`
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	SparkUIPort                             int                      `mapstructure:"spark_ui_port"`
	Metrics                                 metadata.MetricsSettings `mapstructure:"metrics"`
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if c.InstanceName == "" {
		return errors.New("instance_name is required")
	}
	if c.Token == "" {
		return errors.New("token is required")
	}
	return nil
}
