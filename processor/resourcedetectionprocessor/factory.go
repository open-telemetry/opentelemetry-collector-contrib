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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/gce"
)

const (
	// The value of "type" key in configuration.
	typeStr = "resourcedetection"
)

// Factory is the factory for resourcedetection processor.
type Factory struct {
	resourceProviderFactory *internal.ResourceProviderFactory

	// providers stores a provider for each named processor that
	// may a different set of detectors configured.
	providers map[string]*internal.ResourceProvider
	lock      sync.Mutex
}

// NewFactory creates a new factory for resourcedetection processor.
func NewFactory() *Factory {
	resourceProviderFactory := internal.NewProviderFactory(map[internal.DetectorType]internal.Detector{
		env.TypeStr: &env.Detector{},
		gce.TypeStr: gce.NewDetector(),
	})

	return &Factory{
		resourceProviderFactory: resourceProviderFactory,
		providers:               map[string]*internal.ResourceProvider{},
	}
}

// Type gets the type of the Option config created by this factory.
func (*Factory) Type() configmodels.Type {
	return typeStr
}

// CreateDefaultConfig creates the default configuration for processor.
func (*Factory) CreateDefaultConfig() configmodels.Processor {
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

// CreateTraceProcessor creates a trace processor based on this config.
func (f *Factory) CreateTraceProcessor(
	ctx context.Context,
	params component.ProcessorCreateParams,
	nextConsumer consumer.TraceConsumer,
	cfg configmodels.Processor,
) (component.TraceProcessor, error) {
	oCfg := cfg.(*Config)

	provider, err := f.getResourceProvider(ctx, params.Logger, cfg.Name(), oCfg.Timeout, oCfg.Detectors)
	if err != nil {
		return nil, err
	}

	return newResourceTraceProcessor(ctx, nextConsumer, provider, oCfg.Override), nil
}

// CreateMetricsProcessor creates a metrics processor based on this config.
func (f *Factory) CreateMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateParams,
	nextConsumer consumer.MetricsConsumer,
	cfg configmodels.Processor,
) (component.MetricsProcessor, error) {
	oCfg := cfg.(*Config)

	provider, err := f.getResourceProvider(ctx, params.Logger, cfg.Name(), oCfg.Timeout, oCfg.Detectors)
	if err != nil {
		return nil, err
	}

	return newResourceMetricProcessor(ctx, nextConsumer, provider, oCfg.Override), nil
}

func (f *Factory) getResourceProvider(
	ctx context.Context,
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
