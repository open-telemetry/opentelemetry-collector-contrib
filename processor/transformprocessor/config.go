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

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"go.opentelemetry.io/collector/config"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

type LogsConfig struct {
	Queries []string `mapstructure:"queries"`

	// The functions that have been registered in the extension for logs processing.
	functions map[string]interface{} `mapstructure:"-"`
}

type TracesConfig struct {
	Queries []string `mapstructure:"queries"`

	// The functions that have been registered in the extension for traces processing.
	functions map[string]interface{} `mapstructure:"-"`
}

type Config struct {
	config.ProcessorSettings `mapstructure:",squash"`

	Logs   LogsConfig   `mapstructure:"logs"`
	Traces TracesConfig `mapstructure:"traces"`
}

var _ config.Processor = (*Config)(nil)

func (c *Config) Validate() error {
	var errors error
	_, err := common.ParseQueries(c.Logs.Queries, c.Logs.functions, logs.ParsePath)
	if err != nil {
		errors = multierr.Append(errors, err)
	}
	_, err = common.ParseQueries(c.Traces.Queries, c.Traces.functions, traces.ParsePath)
	if err != nil {
		errors = multierr.Append(errors, err)
	}
	return errors
}
