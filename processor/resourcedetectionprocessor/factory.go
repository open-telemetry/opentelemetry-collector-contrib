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

package resourcedetectionprocessor

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/gce"
)

const (
	// The value of "type" key in configuration.
	typeStr = "resourcedetection"
)

var processorCapabilities = component.ProcessorCapabilities{MutatesConsumedData: true}

type factory struct {
	resourceProviderFactory *internal.ResourceProviderFactory

	// providers stores a provider for each named processor that
	// may a different set of detectors configured.
	providers map[string]*internal.ResourceProvider
	lock      sync.Mutex
}

// NewFactory creates a new factory for ResourceDetection processor.
func NewFactory() component.ProcessorFactory {
	resourceProviderFactory := internal.NewProviderFactory(map[internal.DetectorType]internal.DetectorFactory{
		env.TypeStr: env.NewDetector,
		gce.TypeStr: gce.NewDetector,
		ec2.TypeStr: ec2.NewDetector,
	})

	f := &factory{
		resourceProviderFactory: resourceProviderFactory,
		providers:               map[string]*internal.ResourceProvider{},
	}

	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithTraces(f.createTraceProcessor),
		processorhelper.WithMetrics(f.createMetricsProcessor),
		processorhelper.WithLogs(f.createLogsProcessor))
}

// Type gets the type of the Option config created by this factory.
func (*factory) Type() configmodels.Type {
	return typeStr
}

func createDefaultConfig() configmodels.Processor {
	return &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		Detectors: []string{env.TypeStr},
		Timeout:   5 * time.Second,
		Override:  true,
	}
}

func (f *factory) createTraceProcessor(
	_ context.Context,
	params component.ProcessorCreateParams,
	cfg configmodels.Processor,
	nextConsumer consumer.TracesConsumer,
) (component.TraceProcessor, error) {
	rdp, err := f.getResourceDetectionProcessor(params.Logger, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTraceProcessor(
		cfg,
		nextConsumer,
		rdp,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func (f *factory) createMetricsProcessor(
	_ context.Context,
	params component.ProcessorCreateParams,
	cfg configmodels.Processor,
	nextConsumer consumer.MetricsConsumer,
) (component.MetricsProcessor, error) {
	rdp, err := f.getResourceDetectionProcessor(params.Logger, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetricsProcessor(
		cfg,
		nextConsumer,
		rdp,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func (f *factory) createLogsProcessor(
	_ context.Context,
	params component.ProcessorCreateParams,
	cfg configmodels.Processor,
	nextConsumer consumer.LogsConsumer,
) (component.LogsProcessor, error) {
	rdp, err := f.getResourceDetectionProcessor(params.Logger, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogsProcessor(
		cfg,
		nextConsumer,
		rdp,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func (f *factory) getResourceDetectionProcessor(
	logger *zap.Logger,
	cfg configmodels.Processor,
) (*resourceDetectionProcessor, error) {
	oCfg := cfg.(*Config)

	provider, err := f.getResourceProvider(logger, cfg.Name(), oCfg.Timeout, oCfg.Detectors)
	if err != nil {
		return nil, err
	}

	return &resourceDetectionProcessor{
		provider: provider,
		override: oCfg.Override,
	}, nil
}

func (f *factory) getResourceProvider(
	logger *zap.Logger,
	processorName string,
	timeout time.Duration,
	configuredDetectors []string,
) (*internal.ResourceProvider, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if provider, ok := f.providers[processorName]; ok {
		return provider, nil
	}

	detectorTypes := make([]internal.DetectorType, 0, len(configuredDetectors))
	for _, key := range configuredDetectors {
		detectorTypes = append(detectorTypes, internal.DetectorType(strings.TrimSpace(key)))
	}

	provider, err := f.resourceProviderFactory.CreateResourceProvider(logger, timeout, detectorTypes...)
	if err != nil {
		return nil, err
	}

	f.providers[processorName] = provider
	return provider, nil
}
