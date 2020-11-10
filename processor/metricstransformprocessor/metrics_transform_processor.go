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
	"fmt"
	"regexp"
	"strconv"
	"strings"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type metricsTransformProcessor struct {
	transforms []internalTransform
	logger     *zap.Logger
}

var _ processorhelper.MProcessor = (*metricsTransformProcessor)(nil)

type internalTransform struct {
	MetricIncludeFilter internalFilter
	Action              ConfigAction
	NewName             string
	AggregationType     AggregationType
	SubmatchCase        SubmatchCase
	Operations          []internalOperation
}

type internalOperation struct {
	configOperation     Operation
	valueActionsMapping map[string]string
	labelSetMap         map[string]bool
	aggregatedValuesSet map[string]bool
}

type internalFilter interface {
	getMatches(toMatch metricNameMapping) []*match
	getSubexpNames() []string
}

type match struct {
	metric     *metricspb.Metric
	pattern    *regexp.Regexp
	submatches []int
}

type internalFilterStrict struct {
	include string
}

func (f internalFilterStrict) getMatches(toMatch metricNameMapping) []*match {
	if metrics, ok := toMatch[f.include]; ok {
		matches := make([]*match, len(metrics))
		for i, metric := range metrics {
			matches[i] = &match{metric: metric}
		}
		return matches
	}

	return nil
}

func (f internalFilterStrict) getSubexpNames() []string {
	return nil
}

type internalFilterRegexp struct {
	include *regexp.Regexp
}

func (f internalFilterRegexp) getMatches(toMatch metricNameMapping) []*match {
	matches := make([]*match, 0, 10)
	for name, metrics := range toMatch {
		if submatches := f.include.FindStringSubmatchIndex(name); submatches != nil {
			for _, metric := range metrics {
				matches = append(matches, &match{metric: metric, pattern: f.include, submatches: submatches})
			}
		}
	}
	return matches
}

func (f internalFilterRegexp) getSubexpNames() []string {
	return f.include.SubexpNames()
}

type metricNameMapping map[string][]*metricspb.Metric

func newMetricNameMapping(data *consumerdata.MetricsData) metricNameMapping {
	mnm := metricNameMapping(make(map[string][]*metricspb.Metric, len(data.Metrics)))
	for _, m := range data.Metrics {
		mnm.add(m.MetricDescriptor.Name, m)
	}
	return mnm
}

func (mnm metricNameMapping) add(name string, metrics ...*metricspb.Metric) {
	mnm[name] = append(mnm[name], metrics...)
}

func (mnm metricNameMapping) remove(name string, metrics ...*metricspb.Metric) {
	for _, metric := range metrics {
		for j, m := range mnm[name] {
			if metric == m {
				mnm[name] = append(mnm[name][:j], mnm[name][j+1:]...)
				break
			}
		}
	}
}

func newMetricsTransformProcessor(logger *zap.Logger, internalTransforms []internalTransform) *metricsTransformProcessor {
	return &metricsTransformProcessor{
		transforms: internalTransforms,
		logger:     logger,
	}
}

// ProcessMetrics implements the MProcessor interface.
func (mtp *metricsTransformProcessor) ProcessMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	mds := internaldata.MetricsToOC(md)

	for i := range mds {
		data := &mds[i]

		nameToMetricMapping := newMetricNameMapping(data)
		for _, transform := range mtp.transforms {
			matchedMetrics := transform.MetricIncludeFilter.getMatches(nameToMetricMapping)

			if transform.Action == Combine && len(matchedMetrics) > 0 {
				if err := mtp.canBeCombined(matchedMetrics); err != nil {
					// TODO: report via trace / metric instead
					mtp.logger.Warn(err.Error())
					continue
				}

				combined := mtp.combine(matchedMetrics, transform)
				data.Metrics = mtp.removeMatchedMetricsAndAppendCombined(data.Metrics, matchedMetrics, combined)

				// set matchedMetrics to the combined metric so that any additional operations are performed on the combined metric
				matchedMetrics = []*match{{metric: combined}}
			}

			for _, match := range matchedMetrics {
				metricName := match.metric.MetricDescriptor.Name

				if transform.Action == Insert {
					match.metric = proto.Clone(match.metric).(*metricspb.Metric)
					data.Metrics = append(data.Metrics, match.metric)
				}

				mtp.update(match, transform)

				if transform.NewName != "" {
					if transform.Action == Update {
						nameToMetricMapping.remove(metricName, match.metric)
					}
					nameToMetricMapping.add(match.metric.MetricDescriptor.Name, match.metric)
				}
			}
		}
	}

	return internaldata.OCSliceToMetrics(mds), nil
}

// canBeCombined returns true if all the provided metrics share the same type, unit, and labels
func (mtp *metricsTransformProcessor) canBeCombined(matchedMetrics []*match) error {
	if len(matchedMetrics) <= 1 {
		return nil
	}

	firstMetricDescriptor := matchedMetrics[0].metric.MetricDescriptor
	firstMetricLabelKeys := make(map[string]struct{}, len(firstMetricDescriptor.LabelKeys))
	for _, label := range firstMetricDescriptor.LabelKeys {
		firstMetricLabelKeys[label.Key] = struct{}{}
	}

	for i := 1; i < len(matchedMetrics); i++ {
		metric := matchedMetrics[i].metric
		if metric.MetricDescriptor.Type != firstMetricDescriptor.Type {
			return fmt.Errorf("metrics cannot be combined as they are of different types: %v (%v) and %v (%v)", firstMetricDescriptor.Name, firstMetricDescriptor.Type, metric.MetricDescriptor.Name, metric.MetricDescriptor.Type)
		}
		if metric.MetricDescriptor.Unit != firstMetricDescriptor.Unit {
			return fmt.Errorf("metrics cannot be combined as they have different units: %v (%v) and %v (%v)", firstMetricDescriptor.Name, firstMetricDescriptor.Unit, metric.MetricDescriptor.Name, metric.MetricDescriptor.Unit)
		}

		if len(metric.MetricDescriptor.LabelKeys) != len(firstMetricLabelKeys) {
			return fmt.Errorf("metrics cannot be combined as they have different labels: %v (%v) and %v (%v)", firstMetricDescriptor.Name, firstMetricDescriptor.LabelKeys, metric.MetricDescriptor.Name, metric.MetricDescriptor.LabelKeys)
		}

		for _, label := range metric.MetricDescriptor.LabelKeys {
			if _, ok := firstMetricLabelKeys[label.Key]; !ok {
				return fmt.Errorf("metrics cannot be combined as they have different labels: %v (%v) and %v (%v)", firstMetricDescriptor.Name, firstMetricDescriptor.LabelKeys, metric.MetricDescriptor.Name, metric.MetricDescriptor.LabelKeys)
			}
		}
	}

	return nil
}

// combine combines the metrics based on the supplied filter.
func (mtp *metricsTransformProcessor) combine(matchedMetrics []*match, transform internalTransform) *metricspb.Metric {
	// create combined metric with relevant name & descriptor
	combinedMetric := &metricspb.Metric{}
	combinedMetric.MetricDescriptor = proto.Clone(matchedMetrics[0].metric.MetricDescriptor).(*metricspb.MetricDescriptor)
	combinedMetric.MetricDescriptor.Name = transform.NewName
	combinedMetric.MetricDescriptor.Description = ""

	// append label keys based on the transform filter's named capturing groups
	subexprNames := transform.MetricIncludeFilter.getSubexpNames()
	for i := 1; i < len(subexprNames); i++ {
		// if the subexpression is not named, use regexp notation, e.g. $1
		name := subexprNames[i]
		if name == "" {
			name = "$" + strconv.Itoa(i)
		}

		combinedMetric.MetricDescriptor.LabelKeys = append(combinedMetric.MetricDescriptor.LabelKeys, &metricspb.LabelKey{Key: name})
	}

	// combine timeseries from all metrics, using the specified AggregationType if data points need to be merged
	var allTimeseries []*metricspb.TimeSeries
	for _, match := range matchedMetrics {
		for _, ts := range match.metric.Timeseries {
			// append label values based on regex submatches
			for i := 1; i < len(match.submatches)/2; i++ {
				submatch := match.metric.MetricDescriptor.Name[match.submatches[2*i]:match.submatches[2*i+1]]
				submatch = replaceCaseOfSubmatch(transform.SubmatchCase, submatch)
				ts.LabelValues = append(ts.LabelValues, &metricspb.LabelValue{Value: submatch, HasValue: (submatch != "")})
			}

			allTimeseries = append(allTimeseries, match.metric.Timeseries...)
		}
	}

	groupedTimeseries := mtp.groupTimeseries(allTimeseries, len(combinedMetric.MetricDescriptor.LabelKeys))
	aggregatedTimeseries := mtp.mergeTimeseries(groupedTimeseries, transform.AggregationType, combinedMetric.MetricDescriptor.Type)

	mtp.sortTimeseries(aggregatedTimeseries)
	combinedMetric.Timeseries = aggregatedTimeseries

	return combinedMetric
}

func replaceCaseOfSubmatch(replacement SubmatchCase, submatch string) string {
	switch replacement {
	case Lower:
		return strings.ToLower(submatch)
	case Upper:
		return strings.ToUpper(submatch)
	}

	return submatch
}

// removeMatchedMetricsAndAppendCombined removes the set of matched metrics from metrics and appends the combined metric at the end.
func (mtp *metricsTransformProcessor) removeMatchedMetricsAndAppendCombined(metrics []*metricspb.Metric, matchedMetrics []*match, combined *metricspb.Metric) []*metricspb.Metric {
	filteredMetrics := make([]*metricspb.Metric, 0, len(metrics)-len(matchedMetrics))
	for _, metric := range metrics {
		var matched bool
		for _, match := range matchedMetrics {
			if match.metric == metric {
				matched = true
				break
			}
		}
		if !matched {
			filteredMetrics = append(filteredMetrics, metric)
		}
	}

	return append(filteredMetrics, combined)
}

// update updates the metric content based on operations indicated in transform.
func (mtp *metricsTransformProcessor) update(match *match, transform internalTransform) {
	if transform.NewName != "" {
		if match.pattern == nil {
			match.metric.MetricDescriptor.Name = transform.NewName
		} else {
			match.metric.MetricDescriptor.Name = string(match.pattern.ExpandString([]byte{}, transform.NewName, match.metric.MetricDescriptor.Name, match.submatches))
		}
	}

	for _, op := range transform.Operations {
		switch op.configOperation.Action {
		case UpdateLabel:
			mtp.updateLabelOp(match.metric, op)
		case AggregateLabels:
			mtp.aggregateLabelsOp(match.metric, op)
		case AggregateLabelValues:
			mtp.aggregateLabelValuesOp(match.metric, op)
		case ToggleScalarDataType:
			mtp.ToggleScalarDataType(match.metric)
		case AddLabel:
			mtp.addLabelOp(match.metric, op)
		case DeleteLabelValue:
			mtp.deleteLabelValueOp(match.metric, op)
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

// maxInt64 returns the max between num1 and num2
func (mtp *metricsTransformProcessor) maxInt64(num1, num2 int64) int64 {
	if num1 > num2 {
		return num1
	}
	return num2
}

// minInt64 returns the min between num1 and num2
func (mtp *metricsTransformProcessor) minInt64(num1, num2 int64) int64 {
	if num1 < num2 {
		return num1
	}
	return num2
}

// compareTimestamps returns if t1 is a smaller timestamp than t2
func (mtp *metricsTransformProcessor) compareTimestamps(t1 *timestamppb.Timestamp, t2 *timestamppb.Timestamp) bool {
	if t1 == nil || t2 == nil {
		return t1 != nil
	}

	return t1.Seconds < t2.Seconds || (t1.Seconds == t2.Seconds && t1.Nanos < t2.Nanos)
}
