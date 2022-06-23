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

package icingareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icingareceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/config"
	"go.uber.org/multierr"
)

type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`

	Host                   string `mapstructure:"host"`
	Username               string `mapstructure:"username"`
	Password               string `mapstructure:"password"`
	DisableSslVerification bool   `mapstructure:"disable_ssl_verification"`
	// Doc: https://icinga.com/docs/icinga-2/latest/doc/12-icinga2-api/#icinga2-api-filters
	Filter string `mapstructure:"filter"`

	Histograms []HistogramConfig `mapstructure:"histograms"`
}

type HistogramConfig struct {
	Service string    `mapstructure:"service"`
	Host    string    `mapstructure:"host"`
	Type    string    `mapstructure:"type"`
	Values  []float64 `mapstructure:"values"`
}

func (c *Config) Validate() error {
	var errs error

	if c.Host == "" {
		errs = multierr.Append(errs, errors.New("host not specified"))
	}
	if c.Username == "" {
		errs = multierr.Append(errs, errors.New("username not specified"))
	}

	for _, histogram := range c.Histograms {
		previous := 0.
		for _, v := range histogram.Values {
			if v <= previous {
				errs = multierr.Append(errs, fmt.Errorf("histogram with service=\"%s\", host=\"%s\", type=\"%s\" does not have strictly increasing values", histogram.Service, histogram.Host, histogram.Type))
				break
			} else {
				previous = v
			}
		}
	}

	return errs
}
