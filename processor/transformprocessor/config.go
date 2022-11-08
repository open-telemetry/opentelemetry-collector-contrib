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

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllogs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottltraces"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

type Config struct {
	config.ProcessorSettings `mapstructure:",squash"`

	OTTLConfig `mapstructure:",squash"`
}

type OTTLConfig struct {
	Traces  SignalConfig `mapstructure:"traces"`
	Metrics SignalConfig `mapstructure:"metrics"`
	Logs    SignalConfig `mapstructure:"logs"`
}

type SignalConfig struct {
	Statements []string `mapstructure:"statements"`
}

var _ component.ProcessorConfig = (*Config)(nil)

func (c *Config) Validate() error {
	var errors error

	ottltracesp := ottltraces.NewParser(traces.Functions(), component.TelemetrySettings{Logger: zap.NewNop()})
	_, err := ottltracesp.ParseStatements(c.Traces.Statements)
	if err != nil {
		errors = multierr.Append(errors, err)
	}

	ottlmetricsp := ottldatapoints.NewParser(metrics.Functions(), component.TelemetrySettings{Logger: zap.NewNop()})
	_, err = ottlmetricsp.ParseStatements(c.Metrics.Statements)
	if err != nil {
		errors = multierr.Append(errors, err)
	}

	ottllogsp := ottllogs.NewParser(logs.Functions(), component.TelemetrySettings{Logger: zap.NewNop()})
	_, err = ottllogsp.ParseStatements(c.Logs.Statements)
	if err != nil {
		errors = multierr.Append(errors, err)
	}
	return errors
}
