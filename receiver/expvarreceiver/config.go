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

package expvarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver"

import (
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	HTTP                                    *confighttp.HTTPClientSettings `mapstructure:"http_client"`
	MetricsConfig                           []MetricConfig                 `mapstructure:"metrics"`
}

type MetricConfig struct {
	Name    string `mapstructure:"name"`
	Enabled bool   `mapstructure:"enabled"`
}

var _ config.Receiver = (*Config)(nil)

func (c *Config) Validate() error {
	if c.HTTP == nil {
		return fmt.Errorf("must specify http_client configuration when using expvar receiver")
	}
	u, err := url.Parse(c.HTTP.Endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is not a valid URL: %v", err)
	}
	if u.Host == "" {
		return fmt.Errorf("host not found in HTTP endpoint")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be 'http' or 'https', but was '%s'", u.Scheme)
	}
	return nil
}
