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

package logzioexporter

import (
	"errors"

	"go.opentelemetry.io/collector/config/configmodels"
)

type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"`
	Token                         string `mapstructure:"account_token"`
	Region                        string `mapstructure:"region"`
	CustomListenerAddress         string `mapstructure:"custom_listener_address"`
}

func (c *Config) validate() error {
	if c.Token == "" {
		return errors.New("`Account Token` not specified")
	}
	return nil
}
