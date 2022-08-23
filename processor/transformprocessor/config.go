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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/tqllogs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/tqlmetrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/tqltraces"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tqlconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

type Config struct {
	config.ProcessorSettings `mapstructure:",squash"`

	tqlconfig.Config `mapstructure:",squash"`
}

var _ config.Processor = (*Config)(nil)

func (c *Config) Validate() error {
	var errors error
	logger, _ := zap.NewProduction()

	tqlp := tql.Parser{
		Functions:  traces.Functions(),
		PathParser: tqltraces.ParsePath,
		EnumParser: tqltraces.ParseEnum,
		Logger:     logger,
	}
	_, err := tqlp.ParseQueries(c.Traces.Queries)
	if err != nil {
		errors = multierr.Append(errors, err)
	}

	tqlp = tql.Parser{
		Functions:  metrics.Functions(),
		PathParser: tqlmetrics.ParsePath,
		EnumParser: tqlmetrics.ParseEnum,
		Logger:     logger,
	}
	_, err = tqlp.ParseQueries(c.Metrics.Queries)
	if err != nil {
		errors = multierr.Append(errors, err)
	}

	tqlp = tql.Parser{
		Functions:  logs.Functions(),
		PathParser: tqllogs.ParsePath,
		EnumParser: tqllogs.ParseEnum,
		Logger:     logger,
	}
	_, err = tqlp.ParseQueries(c.Logs.Queries)
	if err != nil {
		errors = multierr.Append(errors, err)
	}
	return errors
}
