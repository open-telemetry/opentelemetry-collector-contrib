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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ecs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/elasticbeanstalk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/aks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/gce"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/gke"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"
)

const (
	// The value of "type" key in configuration.
	typeStr = "resourcedetection"
)

var consumerCapabilities = consumer.Capabilities{MutatesData: true}

type factory struct {
	resourceProviderFactory *internal.ResourceProviderFactory

	// providers stores a provider for each named processor that
	// may a different set of detectors configured.
	providers map[config.ComponentID]*internal.ResourceProvider
	lock      sync.Mutex
}

// NewFactory creates a new factory for ResourceDetection processor.
func NewFactory() component.ProcessorFactory {
	resourceProviderFactory := internal.NewProviderFactory(map[internal.DetectorType]internal.DetectorFactory{
		aks.TypeStr:              aks.NewDetector,
		azure.TypeStr:            azure.NewDetector,
		ec2.TypeStr:              ec2.NewDetector,
		ecs.TypeStr:              ecs.NewDetector,
		eks.TypeStr:              eks.NewDetector,
		elasticbeanstalk.TypeStr: elasticbeanstalk.NewDetector,
		env.TypeStr:              env.NewDetector,
		gce.TypeStr:              gce.NewDetector,
		gke.TypeStr:              gke.NewDetector,
		system.TypeStr:           system.NewDetector,
	})

	f := &factory{
		resourceProviderFactory: resourceProviderFactory,
		providers:               map[config.ComponentID]*internal.ResourceProvider{},
	}

	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithTraces(f.createTracesProcessor),
		processorhelper.WithMetrics(f.createMetricsProcessor),
		processorhelper.WithLogs(f.createLogsProcessor))
}

// Type gets the type of the Option config created by this factory.
func (*factory) Type() config.Type {
	return typeStr
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewID(typeStr)),
		Detectors:         []string{env.TypeStr},
		Timeout:           5 * time.Second,
		Override:          true,
	}
}

func (f *factory) createTracesProcessor(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {
	rdp, err := f.getResourceDetectionProcessor(params, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTracesProcessor(
		cfg,
		nextConsumer,
		rdp.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func (f *factory) createMetricsProcessor(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	rdp, err := f.getResourceDetectionProcessor(params, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetricsProcessor(
		cfg,
		nextConsumer,
		rdp.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func (f *factory) createLogsProcessor(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Logs,
) (component.LogsProcessor, error) {
	rdp, err := f.getResourceDetectionProcessor(params, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogsProcessor(
		cfg,
		nextConsumer,
		rdp.processLogs,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func (f *factory) getResourceDetectionProcessor(
	params component.ProcessorCreateSettings,
	cfg config.Processor,
) (*resourceDetectionProcessor, error) {
	oCfg := cfg.(*Config)

	provider, err := f.getResourceProvider(params, cfg.ID(), oCfg.Timeout, oCfg.Detectors, oCfg.DetectorConfig)
	if err != nil {
		return nil, err
	}

	return &resourceDetectionProcessor{
		provider: provider,
		override: oCfg.Override,
	}, nil
}

func (f *factory) getResourceProvider(
	params component.ProcessorCreateSettings,
	processorName config.ComponentID,
	timeout time.Duration,
	configuredDetectors []string,
	detectorConfigs DetectorConfig,
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

	provider, err := f.resourceProviderFactory.CreateResourceProvider(params, timeout, &detectorConfigs, detectorTypes...)
	if err != nil {
		return nil, err
	}

	f.providers[processorName] = provider
	return provider, nil
}
