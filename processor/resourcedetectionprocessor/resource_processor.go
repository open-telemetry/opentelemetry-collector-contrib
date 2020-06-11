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
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type detectResourceResult struct {
	resource pdata.Resource
	err      error
}

type resourceTraceProcessor struct {
	detectors    []internal.Detector
	resource     pdata.Resource
	override     bool
	timeout      time.Duration
	capabilities component.ProcessorCapabilities
	logger       *zap.Logger
	next         consumer.TraceConsumer
}

func newResourceTraceProcessor(ctx context.Context, logger *zap.Logger, next consumer.TraceConsumer, cfg *Config, allDetectors map[string]internal.Detector) (*resourceTraceProcessor, error) {
	detectors, err := getDetectors(ctx, allDetectors, cfg.Detectors)
	if err != nil {
		return nil, err
	}

	processor := &resourceTraceProcessor{
		detectors:    detectors,
		override:     cfg.Override,
		timeout:      cfg.Timeout,
		capabilities: component.ProcessorCapabilities{MutatesConsumedData: true},
		logger:       logger,
		next:         next,
	}

	return processor, nil
}

// GetCapabilities returns the ProcessorCapabilities assocciated with the resource processor.
func (rtp *resourceTraceProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
}

// Start is invoked during service startup.
func (rtp *resourceTraceProcessor) Start(ctx context.Context, host component.Host) error {
	var err error
	rtp.resource, err = detectResource(ctx, rtp.logger, rtp.timeout, rtp.detectors)
	return err
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
	detectors    []internal.Detector
	resource     pdata.Resource
	override     bool
	timeout      time.Duration
	capabilities component.ProcessorCapabilities
	logger       *zap.Logger
	next         consumer.MetricsConsumer
}

func newResourceMetricProcessor(ctx context.Context, logger *zap.Logger, next consumer.MetricsConsumer, cfg *Config, allDetectors map[string]internal.Detector) (*resourceMetricProcessor, error) {
	detectors, err := getDetectors(ctx, allDetectors, cfg.Detectors)
	if err != nil {
		return nil, err
	}

	processor := &resourceMetricProcessor{
		detectors:    detectors,
		override:     cfg.Override,
		timeout:      cfg.Timeout,
		capabilities: component.ProcessorCapabilities{MutatesConsumedData: true},
		logger:       logger,
		next:         next,
	}

	return processor, nil
}

// GetCapabilities returns the ProcessorCapabilities assocciated with the resource processor.
func (rmp *resourceMetricProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
}

// Start is invoked during service startup.
func (rmp *resourceMetricProcessor) Start(ctx context.Context, host component.Host) error {
	var err error
	rmp.resource, err = detectResource(ctx, rmp.logger, rmp.timeout, rmp.detectors)
	return err
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

func detectResource(ctx context.Context, logger *zap.Logger, timeout time.Duration, detectors []internal.Detector) (pdata.Resource, error) {
	var resource pdata.Resource
	ch := make(chan detectResourceResult, 1)

	logger.Info("started detecting resource information")

	go func() {
		res, err := internal.Detect(ctx, detectors...)
		ch <- detectResourceResult{resource: res, err: err}
	}()

	select {
	case <-time.After(timeout):
		return resource, errors.New("timeout attempting to detect resource information")
	case rst := <-ch:
		if rst.err != nil {
			return resource, rst.err
		}

		resource = rst.resource
	}

	logger.Info("completed detecting resource information", zap.Any("resource", resourceToMap(resource)))

	return resource, nil
}

func resourceToMap(resource pdata.Resource) map[string]interface{} {
	mp := make(map[string]interface{}, resource.Attributes().Len())

	resource.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		switch v.Type() {
		case pdata.AttributeValueBOOL:
			mp[k] = v.BoolVal()
		case pdata.AttributeValueINT:
			mp[k] = v.IntVal()
		case pdata.AttributeValueDOUBLE:
			mp[k] = v.DoubleVal()
		case pdata.AttributeValueSTRING:
			mp[k] = v.StringVal()
		}
	})

	return mp
}
