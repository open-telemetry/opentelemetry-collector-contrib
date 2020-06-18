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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type resourceTraceProcessor struct {
	lResource lazyResource
	resource  pdata.Resource
	override  bool
	next      consumer.TraceConsumer
}

func newResourceTraceProcessor(ctx context.Context, next consumer.TraceConsumer, resource lazyResource, override bool) *resourceTraceProcessor {
	return &resourceTraceProcessor{
		lResource: resource,
		override:  override,
		next:      next,
	}
}

// GetCapabilities returns the ProcessorCapabilities assocciated with the resource processor.
func (rtp *resourceTraceProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
}

// Start is invoked during service startup.
func (rtp *resourceTraceProcessor) Start(ctx context.Context, host component.Host) error {
	var err error
	rtp.resource, err = rtp.lResource()
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
	lResource lazyResource
	resource  pdata.Resource
	override  bool
	next      consumer.MetricsConsumer
}

func newResourceMetricProcessor(ctx context.Context, next consumer.MetricsConsumer, resource lazyResource, override bool) *resourceMetricProcessor {
	return &resourceMetricProcessor{
		lResource: resource,
		override:  override,
		next:      next,
	}
}

// GetCapabilities returns the ProcessorCapabilities assocciated with the resource processor.
func (rmp *resourceMetricProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
}

// Start is invoked during service startup.
func (rmp *resourceMetricProcessor) Start(ctx context.Context, host component.Host) error {
	var err error
	rmp.resource, err = rmp.lResource()
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
