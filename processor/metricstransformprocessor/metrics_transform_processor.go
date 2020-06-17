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

package metricstransformprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

type metricsTransformProcessor struct {
	cfg        *Config
	next       consumer.MetricsConsumer
	metricname string
	action     ConfigAction
	newname    string
	operations []Operation
}

var _ component.MetricsProcessor = (*metricsTransformProcessor)(nil)

func newMetricsTransformProcessor(next consumer.MetricsConsumer, cfg *Config) (*metricsTransformProcessor, error) {
	return &metricsTransformProcessor{
		cfg:        cfg,
		next:       next,
		metricname: cfg.MetricName,
		action:     cfg.Action,
		newname:    cfg.NewName,
		operations: cfg.Operations,
	}, nil
}

// GetCapabilities returns the Capabilities assocciated with the resource processor.
func (mtp *metricsTransformProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: false}
}

// Start is invoked during service startup.
func (*metricsTransformProcessor) Start(ctx context.Context, host component.Host) error {
	return nil
}

// Shutdown is invoked during service shutdown.
func (*metricsTransformProcessor) Shutdown(ctx context.Context) error {
	return nil
}

// ConsumeMetrics implements the MetricsProcessor interface
func (mtp *metricsTransformProcessor) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	return mtp.next.ConsumeMetrics(ctx, mtp.transform(md))
}

// transform renames the metrics based off the current and new names specified in the config
func (mtp *metricsTransformProcessor) transform(md pdata.Metrics) pdata.Metrics {
	mds := pdatautil.MetricsToMetricsData(md)

	for _, data := range mds {
		for _, metric := range data.Metrics {
			if mtp.action == Update {
				mtp.update(metric)

			}
		}
	}
	return pdatautil.MetricsFromMetricsData(mds)
}

func (mtp *metricsTransformProcessor) update(metric *metricspb.Metric) {
	if mtp.newname != "" {
		metric.MetricDescriptor.Name = mtp.newname
	}
}