// Copyright  OpenTelemetry Authors
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

package httpdreceiver

import (
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	serverName                              string
}

var (
	defaultProtocol = "http://"
	defaultHost     = "localhost"
	defaultPort     = "8080"
	defaultPath     = "server-status"
	defaultEndpoint = fmt.Sprintf("%s%s:%s/%s?auto", defaultProtocol, defaultHost, defaultPort, defaultPath)
)

func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultEndpoint
		return nil
	}

	if missingProtocol(cfg.Endpoint) {
		cfg.Endpoint = fmt.Sprintf("%s%s", defaultProtocol, cfg.Endpoint)
	}

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint '%s': %w", cfg.Endpoint, err)
	}

	if u.Hostname() == "" {
		u.Host = fmt.Sprintf("%s:%s", defaultHost, defaultPort)
	}

	if u.Port() == "" {
		u.Host = fmt.Sprintf("%s:%s", u.Host, defaultPort)
	}

	if u.Path == "" {
		u.Path = defaultPath
	}

	u.RawQuery = "auto"

	cfg.Endpoint = u.String()
	cfg.serverName = u.Hostname()
	return nil
}

// missingProtocol returns true if any http protocol is found, case sensitive.
func missingProtocol(rawURL string) bool {
	return !strings.HasPrefix(strings.ToLower(rawURL), "http")
}
