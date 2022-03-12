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
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
)

const (
	attributeAWSCloudWatchMetricStreamName = "aws.cloudwatch.metric_stream_name"
	dimensionInstanceID                    = "InstanceId"
	namespaceDelimiter                     = "/"
)

// resourceAttributes are the CloudWatch metric stream attributes that define a
// unique resource.
type resourceAttributes struct {
	// metricStreamName is the metric stream name.
	metricStreamName string
	// accountID is the AWS account ID.
	accountID string
	// region is the AWS region.
	region string
	// namespace is the CloudWatch metric namespace.
	namespace string
}

// The resourceMetricsBuilder is used to aggregate metrics for the
// same resourceAttributes.
type resourceMetricsBuilder struct {
	resourceAttributes `mapstructure:",squash"`
	// metricBuilders is the map of metrics within the same
	// resource group.
	metricBuilders map[string]*metricBuilder
}

// newResourceMetricsBuilder creates a resourceMetricsBuilder with the
// resourceAttributes.
func newResourceMetricsBuilder(attrs resourceAttributes) *resourceMetricsBuilder {
	return &resourceMetricsBuilder{
		resourceAttributes: attrs,
		metricBuilders:     make(map[string]*metricBuilder),
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
func (rmb *resourceMetricsBuilder) setAttributes(resource pdata.Resource) {
	attributes := resource.Attributes()
	attributes.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attributes.InsertString(conventions.AttributeCloudAccountID, rmb.accountID)
	attributes.InsertString(conventions.AttributeCloudRegion, rmb.region)
	serviceNamespace, serviceName := rmb.toServiceAttributes(rmb.namespace)
	if serviceNamespace != "" {
		attributes.InsertString(conventions.AttributeServiceNamespace, serviceNamespace)
	}
	attributes.InsertString(conventions.AttributeServiceName, serviceName)
	attributes.InsertString(attributeAWSCloudWatchMetricStreamName, rmb.metricStreamName)
}

// toServiceAttributes splits the CloudWatch namespace into service namespace/name
// if prepended by AWS/. Otherwise, it returns the CloudWatch namespace as the
// service name with an empty service namespace
func (rmb *resourceMetricsBuilder) toServiceAttributes(namespace string) (serviceNamespace, serviceName string) {
	index := strings.Index(namespace, namespaceDelimiter)
	if index != -1 && strings.EqualFold(namespace[:index], conventions.AttributeCloudProviderAWS) {
		return namespace[:index], namespace[index+1:]
	}
	return "", namespace
}

// dataPointKey combines the dimensions and timestamps to create a key
// used to prevent duplicate metrics.
type dataPointKey struct {
	// timestamp is the milliseconds since epoch
	timestamp int64
	// dimensions is the string representation of the metric dimensions.
	// fmt guarantees key-sorted order when printing a map.
	dimensions string
}

// The metricBuilder aggregates metrics of the same name and unit
// into data points.
type metricBuilder struct {
	// name is the metric name.
	name string
	// unit is the metric unit.
	unit string
	// dataPoints is the slice of summary data points
	// for the metric.
	dataPoints pdata.SummaryDataPointSlice
	// seen is the set of added data point keys.
	seen map[dataPointKey]bool
}

// newMetricBuilder creates a metricBuilder with the name and unit.
func newMetricBuilder(name, unit string) *metricBuilder {
	return &metricBuilder{
		name:       name,
		unit:       unit,
		dataPoints: pdata.NewSummaryDataPointSlice(),
		seen:       make(map[dataPointKey]bool),
	}
}

// AddDataPoint adds the metric as a datapoint if a metric for that timestamp
// hasn't already been added.
func (mb *metricBuilder) AddDataPoint(metric cWMetric) {
	key := dataPointKey{
		timestamp:  metric.Timestamp,
		dimensions: fmt.Sprint(metric.Dimensions),
	}
	if _, ok := mb.seen[key]; !ok {
		mb.toDataPoint(mb.dataPoints.AppendEmpty(), metric)
		mb.seen[key] = true
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

// ToSemConvAttributeKey maps some common keys to semantic convention attributes.
func ToSemConvAttributeKey(key string) string {
	switch key {
	case dimensionInstanceID:
		return conventions.AttributeServiceInstanceID
	default:
		return key
	}
}
