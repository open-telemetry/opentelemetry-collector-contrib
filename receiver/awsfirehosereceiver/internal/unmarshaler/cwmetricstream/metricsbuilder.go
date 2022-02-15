// Copyright  The OpenTelemetry Authors
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

package cwmetricstream // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"
import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

const (
	attributeAWSCloudWatchMetricStreamName = "aws.cloudwatch.metric_stream_name"
	dimensionInstanceID                    = "InstanceId"
	namespaceDelimiter                     = "/"
)

// The resourceMetricsBuilder is used to aggregate metrics for the
// same metric stream name, account ID, region, and namespace.
type resourceMetricsBuilder struct {
	// metricStreamName is the metric stream name.
	metricStreamName string
	// accountID is the AWS account ID.
	accountID string
	// region is the AWS region.
	region string
	// namespace is the CloudWatch metric namespace.
	namespace string
	// metricBuilders is the map of metrics within the same
	// resource group.
	metricBuilders map[string]*metricBuilder
}

// newResourceMetricsBuilder creates a resourceMetricsBuilder for the
// metric stream name, account ID, region, and namespace.
func newResourceMetricsBuilder(metricStreamName, accountID, region, namespace string) *resourceMetricsBuilder {
	return &resourceMetricsBuilder{
		metricStreamName: metricStreamName,
		accountID:        accountID,
		region:           region,
		namespace:        namespace,
		metricBuilders:   make(map[string]*metricBuilder),
	}
}

// AddMetric adds a metric to one of the metric builders based on
// the key generated for each.
func (rmb *resourceMetricsBuilder) AddMetric(metric cWMetric) {
	mb, ok := rmb.metricBuilders[metric.MetricName]
	if !ok {
		mb = newMetricBuilder(metric.MetricName, metric.Unit)
		rmb.metricBuilders[metric.MetricName] = mb
	}
	mb.AddDataPoint(metric)
}

// Build updates the passed in pdata.ResourceMetrics with the metrics in
// the builder.
func (rmb *resourceMetricsBuilder) Build(rm pdata.ResourceMetrics) {
	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	rmb.setAttributes(rm.Resource())
	for _, mb := range rmb.metricBuilders {
		mb.Build(ilm.Metrics().AppendEmpty())
	}
}

// setAttributes creates a pdata.Resource from the fields in the resourceMetricsBuilder.
// Splits the namespace into service.namespace/service.name if prepended by AWS/.
func (rmb *resourceMetricsBuilder) setAttributes(resource pdata.Resource) {
	attributes := resource.Attributes()
	attributes.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attributes.InsertString(conventions.AttributeCloudAccountID, rmb.accountID)
	attributes.InsertString(conventions.AttributeCloudRegion, rmb.region)
	splitNamespace := strings.SplitN(rmb.namespace, namespaceDelimiter, 2)
	if len(splitNamespace) == 2 && strings.EqualFold(splitNamespace[0], conventions.AttributeCloudProviderAWS) {
		attributes.InsertString(conventions.AttributeServiceNamespace, splitNamespace[0])
		attributes.InsertString(conventions.AttributeServiceName, splitNamespace[1])
	} else {
		attributes.InsertString(conventions.AttributeServiceName, rmb.namespace)
	}
	attributes.InsertString(attributeAWSCloudWatchMetricStreamName, rmb.metricStreamName)
}

// The metricBuilder aggregates metrics of the same name and unit
// into data points. Stores the timestamps for each added metric
// in a set to prevent duplicates.
type metricBuilder struct {
	// name is the metric name.
	name string
	// unit is the metric unit.
	unit string
	// dataPoints is the slice of summary data points
	// for the metric.
	dataPoints pdata.SummaryDataPointSlice
	// timestamps is the set of seen timestamps.
	timestamps map[string]bool
}

// newMetricBuilder creates a metricBuilder with the name and unit.
func newMetricBuilder(name, unit string) *metricBuilder {
	return &metricBuilder{
		name:       name,
		unit:       unit,
		dataPoints: pdata.NewSummaryDataPointSlice(),
		timestamps: make(map[string]bool),
	}
}

// AddDataPoint adds the metric as a datapoint if a metric for that timestamp
// hasn't already been added.
func (mb *metricBuilder) AddDataPoint(metric cWMetric) {
	key := mb.toTimestampKey(metric)
	if _, ok := mb.timestamps[key]; !ok {
		mb.toDataPoint(mb.dataPoints.AppendEmpty(), metric)
		mb.timestamps[key] = true
	}
}

// Build builds the pdata.Metric with the data points that were added
// with AddDataPoint.
func (mb *metricBuilder) Build(metric pdata.Metric) {
	metric.SetName(mb.name)
	metric.SetUnit(mb.unit)
	metric.SetDataType(pdata.MetricDataTypeSummary)
	mb.dataPoints.MoveAndAppendTo(metric.Summary().DataPoints())
}

// toDataPoint converts a cWMetric into a pdata datapoint and attaches the
// dimensions as attributes.
func (mb *metricBuilder) toDataPoint(dp pdata.SummaryDataPoint, metric cWMetric) {
	dp.SetCount(uint64(metric.Value.Count))
	dp.SetSum(metric.Value.Sum)
	qv := dp.QuantileValues()
	min := qv.AppendEmpty()
	min.SetQuantile(0)
	min.SetValue(metric.Value.Min)
	max := qv.AppendEmpty()
	max.SetQuantile(1)
	max.SetValue(metric.Value.Max)
	dp.SetTimestamp(pdata.NewTimestampFromTime(time.UnixMilli(metric.Timestamp)))
	for k, v := range metric.Dimensions {
		dp.Attributes().InsertString(ToSemConvAttributeKey(k), v)
	}
}

// toTimestampKey combines the dimensions and timestamps to create a key
// used to prevent duplicate metrics.
func (mb *metricBuilder) toTimestampKey(metric cWMetric) string {
	return fmt.Sprintf("%v::%v", metric.Dimensions, metric.Timestamp)
}

// ToSemConvAttributeKey maps some common keys to semantic convention attributes.
func ToSemConvAttributeKey(key string) string {
	switch key {
	case dimensionInstanceID:
		return conventions.AttributeServiceInstanceID
	default:
		return key
	}
}
