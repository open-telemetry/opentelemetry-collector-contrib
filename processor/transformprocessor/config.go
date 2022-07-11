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
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"go.opentelemetry.io/collector/config"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

type SignalConfig struct {
	Queries    []string             `mapstructure:"queries"`
	Operations []common.ParsedQuery `mapstructure:"operations"`

	// The functions that have been registered in the extension for processing.
	functions map[string]interface{} `mapstructure:"-"`
}

type Config struct {
	config.ProcessorSettings `mapstructure:",squash"`

	Logs    SignalConfig `mapstructure:"logs"`
	Traces  SignalConfig `mapstructure:"traces"`
	Metrics SignalConfig `mapstructure:"metrics"`
}

var _ config.Processor = (*Config)(nil)

func (c *Config) Validate() error {
	if (c.Traces.Operations != nil && c.Traces.Queries != nil) ||
		(c.Metrics.Operations != nil && c.Metrics.Queries != nil) ||
		(c.Logs.Operations != nil && c.Logs.Queries != nil) {
		return fmt.Errorf("providing both operations and queries is not allowed")
	}
	if (c.Traces.Operations == nil && c.Traces.Queries == nil) ||
		(c.Metrics.Operations == nil && c.Metrics.Queries == nil) ||
		(c.Logs.Operations == nil && c.Logs.Queries == nil) {
		return fmt.Errorf("exactly one of operations or queries must be configured")
	}

	var errors error

	if c.Traces.Operations != nil {
		_, err := common.InterpretQueries(c.Traces.Operations, c.Traces.functions, traces.ParsePath)
		errors = multierr.Append(errors, err)
	} else {
		_, err := common.ParseQueries(c.Traces.Queries, c.Traces.functions, traces.ParsePath)
		errors = multierr.Append(errors, err)
	}

	if c.Metrics.Operations != nil {
		_, err := common.InterpretQueries(c.Metrics.Operations, c.Metrics.functions, metrics.ParsePath)
		errors = multierr.Append(errors, err)
	} else {
		_, err := common.ParseQueries(c.Metrics.Queries, c.Metrics.functions, metrics.ParsePath)
		errors = multierr.Append(errors, err)
	}

	if c.Logs.Operations != nil {
		_, err := common.InterpretQueries(c.Logs.Operations, c.Logs.functions, logs.ParsePath)
		errors = multierr.Append(errors, err)
	} else {
		_, err := common.ParseQueries(c.Logs.Queries, c.Logs.functions, logs.ParsePath)
		errors = multierr.Append(errors, err)
	}

	return errors
}
