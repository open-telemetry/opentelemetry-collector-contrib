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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
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

// transform transforms the metrics based on the information specified in the config
func (mtp *metricsTransformProcessor) transform(md pdata.Metrics) pdata.Metrics {
	mds := pdatautil.MetricsToMetricsData(md)

	for i, data := range mds {
		// if the new name is not valid, discard this operation for this list of metrics
		if mtp.validNewName(data.Metrics) {
			for _, metric := range data.Metrics {
				if metric.MetricDescriptor.Name == mtp.metricname {
					// mtp.action is already validated to only contain either update or insert
					if mtp.action == Update {
						mtp.update(metric)
					} else if mtp.action == Insert {
						mds[i].Metrics = mtp.insert(metric, data.Metrics)
					}
				}
			}
		} else {
			log.Printf("error running \"metrics_transform\" processor due to invalid \"new_name\": %v, which might be caused by a collision with existing metric names", mtp.newname)
		}
	}

	return pdatautil.MetricsFromMetricsData(mds)
}

// update updates the original metric content in the metricPtr pointer
func (mtp *metricsTransformProcessor) update(metricPtr *metricspb.Metric) {
	// metric name update
	if mtp.newname != "" {
		metricPtr.MetricDescriptor.Name = mtp.newname
	}

	for _, op := range mtp.operations {
		// update label
		if op.Action == UpdateLabel {
			// label key update
			if op.NewLabel != "" && mtp.validNewLabel(metricPtr.MetricDescriptor.LabelKeys, op.NewLabel) {
				for _, label := range metricPtr.MetricDescriptor.LabelKeys {
					if label.GetKey() == op.Label {
						label.Key = op.NewLabel
					}
				}
			} else {
				log.Printf("error running \"metrics_transform\" processor due to invalid \"new_label\": %v, which might be caused by a collision with existing label on metric named: %v", op.NewLabel, metricPtr.MetricDescriptor.Name)
			}
			//label value update
		}
	}
}

// insert inserts a new copy of the metricPtr content into the metricPtrs slice
func (mtp *metricsTransformProcessor) insert(metricPtr *metricspb.Metric, metricPtrs []*metricspb.Metric) []*metricspb.Metric {
	metricCopy := mtp.createCopy(metricPtr)
	mtp.update(metricCopy)
	return append(metricPtrs, metricCopy)
}

// createCopy creates a new copy of the input metric
func (mtp *metricsTransformProcessor) createCopy(metricPtr *metricspb.Metric) *metricspb.Metric {
	copyMetricDescriptor := *metricPtr.MetricDescriptor
	copyLabelKeys := make([]*metricspb.LabelKey, 0)
	for _, labelKey := range copyMetricDescriptor.LabelKeys {
		copyLabelKeys = append(
			copyLabelKeys,
			&metricspb.LabelKey{
				Key:         labelKey.Key,
				Description: labelKey.Description,
			},
		)
	}
	copyMetricDescriptor.LabelKeys = copyLabelKeys

	copy := &metricspb.Metric{
		MetricDescriptor: &copyMetricDescriptor,
		Timeseries:       metricPtr.Timeseries,
		Resource:         metricPtr.Resource,
	}
	return copy
}

// validNewName determines if the new name is a valid one. An invalid one is one that already exists
func (mtp *metricsTransformProcessor) validNewName(metricPtrs []*metricspb.Metric) bool {
	for _, metric := range metricPtrs {
		if metric.MetricDescriptor.Name == mtp.newname {
			return false
		}
	}
	return true
}

// validNewLabel determines if the new label is a valid one. An invalid one is one that already exists
func (mtp *metricsTransformProcessor) validNewLabel(labelKeys []*metricspb.LabelKey, newLabel string) bool {
	for _, label := range labelKeys {
		if label.Key == newLabel {
			return false
		}
	}
	return true
}
