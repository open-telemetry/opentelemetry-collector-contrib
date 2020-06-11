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
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type resourceTraceProcessor struct {
	resource     pdata.Resource
	override     bool
	capabilities component.ProcessorCapabilities
	next         consumer.TraceConsumer
}

func newResourceTraceProcessor(ctx context.Context, next consumer.TraceConsumer, cfg *Config, allDetectors map[string]internal.Detector) (*resourceTraceProcessor, error) {
	res, err := getResourceUsingDetectors(ctx, allDetectors, cfg.Detectors)
	if err != nil {
		return nil, err
	}

	processor := &resourceTraceProcessor{
		resource:     res,
		override:     cfg.Override,
		capabilities: component.ProcessorCapabilities{MutatesConsumedData: !internal.IsEmptyResource(res)},
		next:         next,
	}

	return processor, nil
}

// GetCapabilities returns the ProcessorCapabilities assocciated with the resource processor.
func (rtp *resourceTraceProcessor) GetCapabilities() component.ProcessorCapabilities {
	return rtp.capabilities
}

// Start is invoked during service startup.
func (*resourceTraceProcessor) Start(ctx context.Context, host component.Host) error {
	return nil
}

// Shutdown is invoked during service shutdown.
func (*resourceTraceProcessor) Shutdown(context.Context) error {
	return nil
}

// ConsumeTraces implements the TraceProcessor interface
func (rtp *resourceTraceProcessor) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	rs := traces.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		internal.MergeResource(rs.At(i).Resource(), rtp.resource, rtp.override)
	}
	return rtp.next.ConsumeTraces(ctx, traces)
}

type resourceMetricProcessor struct {
	resource     pdata.Resource
	override     bool
	capabilities component.ProcessorCapabilities
	next         consumer.MetricsConsumer
}

func newResourceMetricProcessor(ctx context.Context, next consumer.MetricsConsumer, cfg *Config, allDetectors map[string]internal.Detector) (*resourceMetricProcessor, error) {
	res, err := getResourceUsingDetectors(ctx, allDetectors, cfg.Detectors)
	if err != nil {
		return nil, err
	}

	processor := &resourceMetricProcessor{
		resource:     res,
		override:     cfg.Override,
		capabilities: component.ProcessorCapabilities{MutatesConsumedData: !internal.IsEmptyResource(res)},
		next:         next,
	}

	return processor, nil
}

// GetCapabilities returns the ProcessorCapabilities assocciated with the resource processor.
func (rmp *resourceMetricProcessor) GetCapabilities() component.ProcessorCapabilities {
	return rmp.capabilities
}

// Start is invoked during service startup.
func (*resourceMetricProcessor) Start(ctx context.Context, host component.Host) error {
	return nil
}

// Shutdown is invoked during service shutdown.
func (*resourceMetricProcessor) Shutdown(context.Context) error {
	return nil
}

// ConsumeMetrics implements the MetricsProcessor interface
func (rmp *resourceMetricProcessor) ConsumeMetrics(ctx context.Context, metrics pdata.Metrics) error {
	md := pdatautil.MetricsToInternalMetrics(metrics)
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		internal.MergeResource(rm.At(i).Resource(), rmp.resource, rmp.override)
	}
	return rmp.next.ConsumeMetrics(ctx, pdatautil.MetricsFromInternalMetrics(md))
}

func getResourceUsingDetectors(ctx context.Context, detectorsMap map[string]internal.Detector, detectorNames []string) (pdata.Resource, error) {
	detectors, err := getDetectors(ctx, detectorsMap, detectorNames)
	if err != nil {
		return pdata.NewResource(), err
	}

	res, err := getResource(ctx, detectors...)
	if err != nil {
		return pdata.NewResource(), err
	}

	return res, nil
}

func getDetectors(ctx context.Context, allDetectors map[string]internal.Detector, detectorNames []string) ([]internal.Detector, error) {
	detectors := make([]internal.Detector, 0, len(detectorNames))
	for _, key := range detectorNames {
		detector, ok := allDetectors[key]
		if !ok {
			return nil, fmt.Errorf("invalid detector key: %s", key)
		}

		detectors = append(detectors, detector)
	}

	return detectors, nil
}

func getResource(ctx context.Context, detectors ...internal.Detector) (pdata.Resource, error) {
	res, err := internal.Detect(ctx, detectors...)
	if err != nil {
		return res, err
	}

	return res, nil
}
