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

package k8sprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

const (
	// The value of "type" key in configuration.
	typeStr = "k8s_tagger"
)

// Factory is the factory for Attributes processor.
type Factory struct {
	// Factory dependencies that can be provided form outside.
	KubeClient kube.ClientProvider
}

// Type gets the type of the config created by this factory.
func (f *Factory) Type() configmodels.Type {
	return configmodels.Type(typeStr)
}

// CreateDefaultConfig creates the default configuration for processor.
func (f *Factory) CreateDefaultConfig() configmodels.Processor {
	return &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
	}
}

// CreateTraceProcessor creates a trace processor based on this config.
func (f *Factory) CreateTraceProcessor(
	ctx context.Context,
	params component.ProcessorCreateParams,
	nextTraceConsumer consumer.TraceConsumer,
	cfg configmodels.Processor,
) (component.TraceProcessor, error) {
	opts := createProcessorOpts(cfg)
	return NewTraceProcessor(params.Logger, nextTraceConsumer, f.KubeClient, opts...)
}

// CreateMetricsProcessor creates a metrics processor based on this config.
func (f *Factory) CreateMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateParams,
	nextMetricsConsumer consumer.MetricsConsumer,
	cfg configmodels.Processor,
) (component.MetricsProcessor, error) {
	opts := createProcessorOpts(cfg)
	return NewMetricsProcessor(params.Logger, nextMetricsConsumer, f.KubeClient, opts...)
}

func createProcessorOpts(cfg configmodels.Processor) []Option {
	oCfg := cfg.(*Config)
	opts := []Option{}
	if oCfg.Passthrough {
		opts = append(opts, WithPassthrough())
	}

	// extraction rules
	opts = append(opts, WithExtractMetadata(oCfg.Extract.Metadata...))
	opts = append(opts, WithExtractLabels(oCfg.Extract.Labels...))
	opts = append(opts, WithExtractAnnotations(oCfg.Extract.Annotations...))

	// filters
	opts = append(opts, WithFilterNode(oCfg.Filter.Node, oCfg.Filter.NodeFromEnvVar))
	opts = append(opts, WithFilterNamespace(oCfg.Filter.Namespace))
	opts = append(opts, WithFilterLabels(oCfg.Filter.Labels...))
	opts = append(opts, WithFilterFields(oCfg.Filter.Fields...))
	opts = append(opts, WithAPIConfig(oCfg.APIConfig))

	return opts
}
