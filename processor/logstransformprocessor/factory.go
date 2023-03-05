// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstransformprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const (
	// The value of "type" key in configuration.
	typeStr = "logstransform"
	// The stability level of the processor.
	stability = component.StabilityLevelDevelopment
)

// NewFactory returns a new factory for the Logs Transform processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, stability))
}

// Note: This isn't a valid configuration because the processor would do no work.
func createDefaultConfig() component.Config {
	return &Config{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
	}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs) (processor.Logs, error) {
	pCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("could not initialize logs transform processor")
	}

	if len(pCfg.BaseConfig.Operators) == 0 {
		return nil, errors.New("no operators were configured for this logs transform processor")
	}

	return newProcessor(pCfg, nextConsumer, set.Logger)
}
