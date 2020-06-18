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
	detectors map[string]internal.Detector
	resources map[string]lazyResource
	lock      sync.Mutex
}

// NewFactory creates a new factory for resourcedetection processor.
func NewFactory() *Factory {
	return &Factory{
		detectors: map[string]internal.Detector{
			env.TypeStr: &env.Detector{},
			gce.TypeStr: gce.NewDetector(),
		},
		resources: map[string]lazyResource{},
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

	lResource, err := f.getLazyResourceForProcessor(ctx, params.Logger, cfg.Name(), oCfg.Detectors, oCfg.Timeout)
	if err != nil {
		return nil, err
	}

	return newResourceTraceProcessor(ctx, nextConsumer, lResource, oCfg.Override), nil
}

// CreateMetricsProcessor creates a metrics processor based on this config.
func (f *Factory) CreateMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateParams,
	nextConsumer consumer.MetricsConsumer,
	cfg configmodels.Processor,
) (component.MetricsProcessor, error) {
	oCfg := cfg.(*Config)

	lResource, err := f.getLazyResourceForProcessor(ctx, params.Logger, cfg.Name(), oCfg.Detectors, oCfg.Timeout)
	if err != nil {
		return nil, err
	}

	return newResourceMetricProcessor(ctx, nextConsumer, lResource, oCfg.Override), nil
}

// getLazyResourceForProcessor returns a function that will lazily detect the
// resource.
//
// The resulting lazy function is cached against the processor name so that
// the resource information will only be detected once even if multiple
// instances of the same processor are created.
func (f *Factory) getLazyResourceForProcessor(ctx context.Context, logger *zap.Logger, processorName string, detectorNames []string, timeout time.Duration) (lazyResource, error) {
	processorDetectors, err := getDetectors(ctx, f.detectors, detectorNames)
	if err != nil {
		return nil, err
	}

	f.lock.Lock()
	defer f.lock.Unlock()

	if lResource, ok := f.resources[processorName]; ok {
		return lResource, nil
	}

	lResource := getLazyResource(ctx, logger, timeout, processorDetectors)
	f.resources[processorName] = lResource
	return lResource, nil
}
