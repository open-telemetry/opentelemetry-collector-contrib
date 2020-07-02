// Copyright 2020 OpenTelemetry Authors
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

package metricstransformprocessor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
)

const (
	// The value of "type" key in configuration.
	typeStr = "metricstransform"
)

// Factory is the factory for metrics transform processor.
type Factory struct{}

// Type gets the type of the Option config created by this factory.
func (f *Factory) Type() configmodels.Type {
	return typeStr
}

// CreateDefaultConfig creates the default configuration for processor.
func (f *Factory) CreateDefaultConfig() configmodels.Processor {
	return &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
	}
}

// CreateTraceProcessor creates a trace processor based on this config.
func (f *Factory) CreateTraceProcessor(
	ctx context.Context,
	params component.ProcessorCreateParams,
	nextConsumer consumer.TraceConsumer,
	cfg configmodels.Processor,
) (component.TraceProcessor, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsProcessor creates a metrics processor based on this config.
func (f *Factory) CreateMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateParams,
	nextConsumer consumer.MetricsConsumer,
	c configmodels.Processor,
) (component.MetricsProcessor, error) {
	oCfg := c.(*Config)
	err := validateConfiguration(oCfg)
	if err != nil {
		return nil, err
	}
	return newMetricsTransformProcessor(nextConsumer, oCfg), nil

}

// validateConfiguration validates the input configuration has all of the required fields for the processor
// and returns a list of valid actions to configure the processor.
// An error is returned if there are any invalid inputs.
func validateConfiguration(config *Config) error {
	for _, transform := range config.Transforms {
		if transform.MetricName == "" {
			return fmt.Errorf("missing required field %q", MetricNameFieldName)
		}

		if transform.Action != Update && transform.Action != Insert {
			return fmt.Errorf("unsupported %q: %v, the supported actions are %q and %q", ActionFieldName, transform.Action, Insert, Update)
		}

		if transform.Action == Insert && transform.NewName == "" {
			return fmt.Errorf("missing required field %q while %q is %v", NewNameFieldName, ActionFieldName, Insert)
		}

		for i, op := range transform.Operations {
			if op.Action == UpdateLabel && op.Label == "" {
				return fmt.Errorf("missing required field %q while %q is %v in the %vth operation", LabelFieldName, ActionFieldName, UpdateLabel, i)
			}
		}
	}

	return nil
}
