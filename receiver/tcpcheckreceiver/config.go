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

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errMissingEndpoint  = errors.New(`"endpoint" not specified in config`)
	errUnknownTransport = errors.New(`"transport" type unknown`)
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	confignet.NetAddr                       `mapstructure:",squash"`

	Timeout              time.Duration                 `mapstructure:"timeout"`
	MetricsBuilderConfig metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

func (c Config) Validate() (err error) {
	if c.Endpoint == "" {
		err = multierr.Append(err, errMissingEndpoint)
	}

	transports := map[string]bool{
		"tcp": true, "tcp4": true, "tcp6": true, "unix": true, "": true,
		// empty string is allowed, because we default it
	}
	if _, ok := transports[c.Transport]; !ok {
		err = multierr.Append(err, errUnknownTransport)
	}

	return
}
