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
	"log"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
)

const (
	validNewNameFuncName  = "validNewName()"
	validNewLabelFuncName = "validNewLabel()"
)

type metricsTransformProcessor struct {
	cfg        *Config
	next       consumer.MetricsConsumer
	transforms []Transform
}

var _ component.MetricsProcessor = (*metricsTransformProcessor)(nil)

func newMetricsTransformProcessor(next consumer.MetricsConsumer, cfg *Config) *metricsTransformProcessor {
	return &metricsTransformProcessor{
		cfg:        cfg,
		next:       next,
		transforms: cfg.Transforms,
	}
}

// GetCapabilities returns the Capabilities associated with the metrics transform processor.
func (mtp *metricsTransformProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
}

// Start is invoked during service startup.
func (*metricsTransformProcessor) Start(ctx context.Context, host component.Host) error {
	return nil
}

// Shutdown is invoked during service shutdown.
func (*metricsTransformProcessor) Shutdown(ctx context.Context) error {
	return nil
}

// ConsumeMetrics implements the MetricsProcessor interface.
func (mtp *metricsTransformProcessor) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	return mtp.next.ConsumeMetrics(ctx, mtp.transform(md))
}

// transform transforms the metrics based on the information specified in the config.
func (mtp *metricsTransformProcessor) transform(md pdata.Metrics) pdata.Metrics {
	mds := pdatautil.MetricsToMetricsData(md)

	for i, data := range mds {
		nameToMetricMapping := make(map[string]*metricspb.Metric)
		for _, metric := range data.Metrics {
			nameToMetricMapping[metric.MetricDescriptor.Name] = metric
		}

		for _, transform := range mtp.transforms {
			if !mtp.validNewName(transform, nameToMetricMapping) {
				log.Printf("error running %q processor due to collision %q: %v with existing metric names detected by the function %q", typeStr, NewNameFieldName, transform.NewName, validNewNameFuncName)
				continue
			}

			metric, ok := nameToMetricMapping[transform.MetricName]
			if !ok {
				continue
			}

			if transform.Action == Update {
				mtp.update(metric, transform)
				if transform.NewName == "" {
					continue
				}
				// if name is updated, the map has to be updated
				nameToMetricMapping[transform.NewName] = nameToMetricMapping[transform.MetricName]
				delete(nameToMetricMapping, transform.MetricName)
			} else if transform.Action == Insert {
				var newMetric *metricspb.Metric
				mds[i].Metrics, newMetric = mtp.insert(metric, mds[i].Metrics, transform)
				// mapping has to be updated with the name metric
				nameToMetricMapping[newMetric.MetricDescriptor.Name] = newMetric
			}
		}
	}

	return pdatautil.MetricsFromMetricsData(mds)
}

// update updates the original metric content in the metric pointer.
func (mtp *metricsTransformProcessor) update(metric *metricspb.Metric, transform Transform) {
	// metric name update
	if transform.NewName != "" {
		metric.MetricDescriptor.Name = transform.NewName
	}

	for _, op := range transform.Operations {
		// update label
		if op.Action == UpdateLabel {
			mtp.updateLabelOp(metric, op)
		}
		// aggregate across labels
		if op.Action == AggregateLabels {
		}
		// aggregate across label values
		if op.Action == AggregateLabelValues {
		}
	}
}

// insert inserts a new copy of the metric content into the metrics slice.
// returns the new metrics list and the new metric
func (mtp *metricsTransformProcessor) insert(metric *metricspb.Metric, metrics []*metricspb.Metric, transform Transform) ([]*metricspb.Metric, *metricspb.Metric) {
	// metricCopy := mtp.createCopy(metric)
	metricCopy := proto.Clone(metric).(*metricspb.Metric)
	mtp.update(metricCopy, transform)
	return append(metrics, metricCopy), metricCopy
}

func (mtp *metricsTransformProcessor) updateLabelOp(metric *metricspb.Metric, op Operation) {
	// if new_label is invalid, skip this operation
	if !mtp.validNewLabel(metric.MetricDescriptor.LabelKeys, op.NewLabel) {
		log.Printf("error running %q processor due to collided %q: %v with existing label on metric named: %v detected by function %q", typeStr, NewLabelFieldName, op.NewLabel, metric.MetricDescriptor.Name, validNewLabelFuncName)
		return
	}

	for _, label := range metric.MetricDescriptor.LabelKeys {
		if label.Key != op.Label {
			continue
		}
		// label key update
		if op.NewLabel != "" {
			label.Key = op.NewLabel
		}
		// label value update
	}
}

// validNewName determines if the new name is a valid one. An invalid one is one that already exists.
func (mtp *metricsTransformProcessor) validNewName(transform Transform, nameToMetricMapping map[string]*metricspb.Metric) bool {
	_, ok := nameToMetricMapping[transform.NewName]
	return !ok
}

// validNewLabel determines if the new label is a valid one. An invalid one is one that already exists.
func (mtp *metricsTransformProcessor) validNewLabel(labelKeys []*metricspb.LabelKey, newLabel string) bool {
	for _, label := range labelKeys {
		if label.Key == newLabel {
			return false
		}
	}
	return true
}
