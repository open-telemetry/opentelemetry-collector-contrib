// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snowflakereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver/internal/metadata"
)

var (
	errMissingUsername  = errors.New("You must provide a valid snowflake username")
	errMissingPassword  = errors.New("You must provide a password for the snowflake username")
	errMissingAccount   = errors.New("You must provide a valid account name")
	errMissingWarehouse = errors.New("You must provide a valid warehouse name")
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
	Username                                string `mapstructure:"username"`
	Password                                string `mapstructure:"password"`
	Account                                 string `mapstructure:"account"`
	Schema                                  string `mapstructure:"schema"`
	Warehouse                               string `mapstructure:"warehouse"`
	Database                                string `mapstructure:"database"`
	Role                                    string `mapstructure:"role"`
}

func (cfg *Config) Validate() error {
	var errs error
	if cfg.Username == "" {
		errs = multierr.Append(errs, errMissingUsername)
	}

	if cfg.Password == "" {
		errs = multierr.Append(errs, errMissingPassword)
	}

	if cfg.Account == "" {
		errs = multierr.Append(errs, errMissingAccount)
	}

	if cfg.Warehouse == "" {
		errs = multierr.Append(errs, errMissingWarehouse)
	}

	return errs
}
