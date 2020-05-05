// Copyright 2019 OpenTelemetry Authors
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

package sourceprocessor

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configerror"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
)

const (
	// The value of "type" key in configuration.
	typeStr                          = "source"
	defaultCollector                 = ""
	defaultSourceName                = "%{namespace}.%{pod}.%{container}"
	defaultSourceCategory            = "%{namespace}/%{pod_name}"
	defaultSourceCategoryPrefix      = "kubernetes/"
	defaultSourceCategoryReplaceDash = "/"
)

// Factory is the factory for OpenCensus exporter.
type Factory struct {
}

var _ component.ProcessorFactory = (*Factory)(nil)

// Type gets the type of the Option config created by this factory.
func (Factory) Type() configmodels.Type {
	return typeStr
}

// CreateDefaultConfig creates the default configuration for processor.
func (Factory) CreateDefaultConfig() configmodels.Processor {
	return &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		Collector:                 defaultCollector,
		SourceName:                defaultSourceName,
		SourceCategory:            defaultSourceCategory,
		SourceCategoryPrefix:      defaultSourceCategoryPrefix,
		SourceCategoryReplaceDash: defaultSourceCategoryReplaceDash,
	}
}

// CreateTraceProcessor creates a trace processor based on this config.
func (f *Factory) CreateTraceProcessor(
	_ context.Context,
	_ component.ProcessorCreateParams,
	nextConsumer consumer.TraceConsumer,
	cfg configmodels.Processor) (component.TraceProcessor, error) {

	oCfg := cfg.(*Config)
	return newSourceTraceProcessor(nextConsumer, oCfg)
}

// CreateMetricsProcessor creates a metric processor based on this config.
func (f *Factory) CreateMetricsProcessor(
	_ context.Context,
	_ component.ProcessorCreateParams,
	_ consumer.MetricsConsumer,
	_ configmodels.Processor) (component.MetricsProcessor, error) {
	// Span Processor does not support Metrics.
	return nil, configerror.ErrDataTypeIsNotSupported
}
