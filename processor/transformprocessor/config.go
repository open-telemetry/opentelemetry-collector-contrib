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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllogs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottltraces"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

type Config struct {
	config.ProcessorSettings `mapstructure:",squash"`

	ottlconfig.Config `mapstructure:",squash"`
}

var _ config.Processor = (*Config)(nil)

func (c *Config) Validate() error {
	var errors error

	ottlp := ottl.NewParser(
		traces.Functions(),
		ottltraces.ParsePath,
		ottltraces.ParseEnum,
		component.TelemetrySettings{},
	)
	_, err := ottlp.ParseStatements(c.Traces.Queries)
	if err != nil {
		errors = multierr.Append(errors, err)
	}

	ottlp = ottl.NewParser(
		metrics.Functions(),
		ottldatapoints.ParsePath,
		ottldatapoints.ParseEnum,
		component.TelemetrySettings{},
	)
	_, err = ottlp.ParseStatements(c.Metrics.Queries)
	if err != nil {
		errors = multierr.Append(errors, err)
	}

	ottlp = ottl.NewParser(
		logs.Functions(),
		ottllogs.ParsePath,
		ottllogs.ParseEnum,
		component.TelemetrySettings{},
	)
	_, err = ottlp.ParseStatements(c.Logs.Queries)
	if err != nil {
		errors = multierr.Append(errors, err)
	}
	return errors
}
