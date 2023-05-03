// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package saphanareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver/internal/metadata"
)

// Errors for missing required config parameters.
const (
	ErrNoUsername = "invalid config: missing username"
	ErrNoPassword = "invalid config: missing password" // #nosec G101 - not hardcoded credentials
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	confignet.TCPAddr                       `mapstructure:",squash"`
	configtls.TLSClientSetting              `mapstructure:"tls,omitempty"`
	// MetricsBuilderConfig defines which metrics/attributes to enable for the scraper
	metadata.MetricsBuilderConfig `mapstructure:",squash"`

	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

func (cfg *Config) Validate() error {
	var err error
	if cfg.Username == "" {
		err = multierr.Append(err, errors.New(ErrNoUsername))
	}
	if cfg.Password == "" {
		err = multierr.Append(err, errors.New(ErrNoPassword))
	}

	return err
}
