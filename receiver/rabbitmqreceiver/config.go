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

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errMissingUsername = errors.New(`"username" not specified in config`)
	errMissingPassword = errors.New(`"password" not specified in config`)

	errInvalidEndpoint = errors.New(`"endpoint" must be in the form of <scheme>://<hostname>:<port>`)
)

const defaultEndpoint = "http://localhost:15672"

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	Username                                string                   `mapstructure:"username"`
	Password                                string                   `mapstructure:"password"`
	Metrics                                 metadata.MetricsSettings `mapstructure:"metrics"`
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *Config) Validate() error {
	var err error
	if cfg.Username == "" {
		err = multierr.Append(err, errMissingUsername)
	}

	if cfg.Password == "" {
		err = multierr.Append(err, errMissingPassword)
	}

	_, parseErr := url.Parse(cfg.Endpoint)
	if parseErr != nil {
		wrappedErr := fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr)
		err = multierr.Append(err, wrappedErr)
	}

	return err
}
