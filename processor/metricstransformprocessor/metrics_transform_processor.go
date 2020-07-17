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

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.uber.org/zap"
)

type mtpTransform struct {
	MetricName string
	Action     ConfigAction
	NewName    string
	Operations []mtpOperation
}

type mtpOperation struct {
	configOperation Operation
	// fields that will be filled later for optimized opertaions processing
	valueActionsMapping map[string]string
	labelSetMap         map[string]bool
	aggregatedValuesSet map[string]bool
}

type metricsTransformProcessor struct {
	next       consumer.MetricsConsumer
	transforms []mtpTransform
	logger     *zap.Logger
}

var _ component.MetricsProcessor = (*metricsTransformProcessor)(nil)

func newMetricsTransformProcessor(next consumer.MetricsConsumer, logger *zap.Logger, mtpTransforms []mtpTransform) *metricsTransformProcessor {
	return &metricsTransformProcessor{
		next:       next,
		transforms: mtpTransforms,
		logger:     logger,
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

	for i := range mds {
		data := &mds[i]
		nameToMetricMapping := make(map[string]*metricspb.Metric, len(data.Metrics))
		for _, metric := range data.Metrics {
			nameToMetricMapping[metric.MetricDescriptor.Name] = metric
		}

		for _, transform := range mtp.transforms {
			metric, ok := nameToMetricMapping[transform.MetricName]
			if !ok {
				continue
			}

			if transform.Action == Insert {
				metric = proto.Clone(metric).(*metricspb.Metric)
				data.Metrics = append(data.Metrics, metric)
			}

			mtp.update(metric, transform)

			if transform.NewName != "" {
				if transform.Action == Update {
					delete(nameToMetricMapping, transform.MetricName)
				}
				nameToMetricMapping[transform.NewName] = metric
			}
		}
	}

	return pdatautil.MetricsFromMetricsData(mds)
}

// // update updates the original metric content in the metric pointer.
// func (mtp *metricsTransformProcessor) update(metric *metricspb.Metric, transform Transform) {
// 	// metric name update
// 	if transform.NewName != "" {
// 		metric.MetricDescriptor.Name = transform.NewName
// 	}

// 	for _, op := range transform.Operations {
// 		// update label
// 		if op.Action == UpdateLabel {
// 			mtp.updateLabelOp(metric, op)
// 		} else if op.Action == ToggleScalarDataType {
// 			mtp.ToggleScalarDataType(metric)
// 		} else if op.Action == AddLabel {
// 			mtp.addLabelOp(metric, op)
// 		}
// 	}
// }

// func (mtp *metricsTransformProcessor) addLabelOp(metric *metricspb.Metric, op Operation) {
// 	var lb = metricspb.LabelKey{
// 		Key: op.NewLabel,
// 	}
// 	metric.MetricDescriptor.LabelKeys = append(metric.MetricDescriptor.LabelKeys, &lb)
// 	for _, ts := range metric.Timeseries {
// 		lv := &metricspb.LabelValue{
// 			Value:    op.NewValue,
// 			HasValue: true,
// 		}
// 		ts.LabelValues = append(ts.LabelValues, lv)
// 	}
// }

// func (mtp *metricsTransformProcessor) updateLabelOp(metric *metricspb.Metric, op Operation) {
// 	for _, label := range metric.MetricDescriptor.LabelKeys {
// 		if label.Key != op.Label {
// 			continue
// 		}
// 		// label key update
// 		if op.NewLabel != "" {
// 			label.Key = op.NewLabel
// 		}
// 		// label value update
// 	}
// }

// func (mtp *metricsTransformProcessor) ToggleScalarDataType(metric *metricspb.Metric) {
// 	for _, ts := range metric.Timeseries {
// 		for _, dp := range ts.Points {
// 			switch metric.MetricDescriptor.Type {
// 			case metricspb.MetricDescriptor_GAUGE_INT64, metricspb.MetricDescriptor_CUMULATIVE_INT64:
// 				dp.Value = &metricspb.Point_DoubleValue{DoubleValue: float64(dp.GetInt64Value())}
// 			case metricspb.MetricDescriptor_GAUGE_DOUBLE, metricspb.MetricDescriptor_CUMULATIVE_DOUBLE:
// 				dp.Value = &metricspb.Point_Int64Value{Int64Value: int64(dp.GetDoubleValue())}
// 			}
// 		}
// 	}

// 	switch metric.MetricDescriptor.Type {
// 	case metricspb.MetricDescriptor_GAUGE_INT64:
// 		metric.MetricDescriptor.Type = metricspb.MetricDescriptor_GAUGE_DOUBLE
// 	case metricspb.MetricDescriptor_CUMULATIVE_INT64:
// 		metric.MetricDescriptor.Type = metricspb.MetricDescriptor_CUMULATIVE_DOUBLE
// 	case metricspb.MetricDescriptor_GAUGE_DOUBLE:
// 		metric.MetricDescriptor.Type = metricspb.MetricDescriptor_GAUGE_INT64
// 	case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE:
// 		metric.MetricDescriptor.Type = metricspb.MetricDescriptor_CUMULATIVE_INT64
// 	}
// }

// update updates the metric content based on operations indicated in transform.
func (mtp *metricsTransformProcessor) update(metric *metricspb.Metric, transform mtpTransform) {
	if transform.NewName != "" {
		metric.MetricDescriptor.Name = transform.NewName
	}

	for _, op := range transform.Operations {
		switch op.configOperation.Action {
		case UpdateLabel:
			mtp.updateLabelOp(metric, op)
		case AggregateLabels:
			mtp.aggregateLabelsOp(metric, op)
		case AggregateLabelValues:
			mtp.aggregateLabelValuesOp(metric, op)
		}
	}
}

// getLabelIdxs gets the indices of the labelSet labels' indices in the metric's descriptor's labels field
// Returns the indices slice and a slice of the actual labels selected by this slice of indices
func (mtp *metricsTransformProcessor) getLabelIdxs(metric *metricspb.Metric, labelSet map[string]bool) ([]int, []*metricspb.LabelKey) {
	labelIdxs := make([]int, 0)
	labels := make([]*metricspb.LabelKey, 0)
	for idx, label := range metric.MetricDescriptor.LabelKeys {
		_, ok := labelSet[label.Key]
		if ok {
			labelIdxs = append(labelIdxs, idx)
			labels = append(labels, label)
		}
	}
	return labelIdxs, labels
}
