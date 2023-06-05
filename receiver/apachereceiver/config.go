// Copyright The OpenTelemetry Authors
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

package apachereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver"

import (
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	MetricsBuilderConfig                    metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

var (
	defaultProtocol = "http://"
	defaultHost     = "localhost"
	defaultPort     = "8080"
	defaultPath     = "server-status"
	defaultEndpoint = fmt.Sprintf("%s%s:%s/%s?auto", defaultProtocol, defaultHost, defaultPort, defaultPath)
)

func (cfg *Config) Validate() error {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint: '%s': %w", cfg.Endpoint, err)
	}

	if u.Hostname() == "" {
		return fmt.Errorf("missing hostname: '%s'", cfg.Endpoint)
	}

	if u.RawQuery != "auto" {
		return fmt.Errorf("query must be 'auto': '%s'", cfg.Endpoint)
	}

	return nil
}
