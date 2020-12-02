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

// Config contains Logz.io specific configuration such as Account TracesToken, Region, etc.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"`
	TracesToken                   string `mapstructure:"account_token"`   // Your Logz.io Account Token, can be found at https://app.logz.io/#/dashboard/settings/general
	MetricsToken                  string `mapstructure:"metrics_token"`   // Your Logz.io Metrics Token, can be found at https://docs.logz.io/user-guide/accounts/finding-your-metrics-account-token/
	Region                        string `mapstructure:"region"`          // Your Logz.io 2-letter region code, can be found at https://docs.logz.io/user-guide/accounts/account-region.html#available-regions
	CustomEndpoint                string `mapstructure:"custom_endpoint"` // Custom endpoint to ship traces to. Use only for dev and tests.
}

func (c *Config) validate() error {
	if c.TracesToken == "" {
		return errors.New("`account_token` not specified")
	}
	return nil
}
