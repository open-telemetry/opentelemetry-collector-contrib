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

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type resourceDetectionProcessor struct {
	provider   *internal.ResourceProvider
	resource   pdata.Resource
	override   bool
	traceNext  consumer.TraceConsumer
	metricNext consumer.MetricsConsumer
	logNext    consumer.LogsConsumer
}

func newResourceDetectionTracesProcessor(_ context.Context, next consumer.TraceConsumer, provider *internal.ResourceProvider, override bool) *resourceDetectionProcessor {
	return &resourceDetectionProcessor{
		provider:  provider,
		override:  override,
		traceNext: next,
	}
}

func newResourceDetectionMetricsProcessor(_ context.Context, next consumer.MetricsConsumer, provider *internal.ResourceProvider, override bool) *resourceDetectionProcessor {
	return &resourceDetectionProcessor{
		provider:   provider,
		override:   override,
		metricNext: next,
	}
}

func newResourceDetectionLogsProcessor(_ context.Context, next consumer.LogsConsumer, provider *internal.ResourceProvider, override bool) *resourceDetectionProcessor {
	return &resourceDetectionProcessor{
		provider: provider,
		override: override,
		logNext:  next,
	}
}

// GetCapabilities returns the capabilities of the processor
func (rdp *resourceDetectionProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
}

// Start is invoked during service startup.
func (rdp *resourceDetectionProcessor) Start(ctx context.Context, host component.Host) error {
	var err error
	rdp.resource, err = rdp.provider.Get(ctx)
	return err
}

// Shutdown is invoked during service shutdown.
func (*resourceDetectionProcessor) Shutdown(context.Context) error {
	return nil
}

// ConsumeTraces implements the TraceProcessor interface
func (rdp *resourceDetectionProcessor) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		res := rs.At(i).Resource()
		if res.IsNil() {
			res.InitEmpty()
		}

		internal.MergeResource(res, rdp.resource, rdp.override)
	}
	return rdp.traceNext.ConsumeTraces(ctx, td)
}

// ConsumeMetrics implements the MetricsProcessor interface
func (rdp *resourceDetectionProcessor) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		res := rm.At(i).Resource()
		if res.IsNil() {
			res.InitEmpty()
		}

		internal.MergeResource(res, rdp.resource, rdp.override)
	}
	return rdp.metricNext.ConsumeMetrics(ctx, md)
}

// ConsumeLogs implements the LogsProcessor interface
func (rdp *resourceDetectionProcessor) ConsumeLogs(ctx context.Context, ld pdata.Logs) error {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		res := rls.At(i).Resource()
		if res.IsNil() {
			res.InitEmpty()
		}

		internal.MergeResource(res, rdp.resource, rdp.override)
	}
	return rdp.logNext.ConsumeLogs(ctx, ld)
}
