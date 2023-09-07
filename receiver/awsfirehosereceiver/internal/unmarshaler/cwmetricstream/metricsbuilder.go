// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwmetricstream // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
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
	rms pmetric.MetricSlice
	// metricBuilders is the map of metrics within the same
	// resource group.
	metricBuilders map[string]*metricBuilder
}

// newResourceMetricsBuilder creates a resourceMetricsBuilder with the
// resourceAttributes.
func newResourceMetricsBuilder(md pmetric.Metrics, attrs resourceAttributes) *resourceMetricsBuilder {
	rms := md.ResourceMetrics().AppendEmpty()
	attrs.setAttributes(rms.Resource())
	return &resourceMetricsBuilder{
		rms:            rms.ScopeMetrics().AppendEmpty().Metrics(),
		metricBuilders: make(map[string]*metricBuilder),
	}
}

// AddMetric adds a metric to one of the metric builders based on
// the key generated for each.
func (rmb *resourceMetricsBuilder) AddMetric(metric cWMetric) {
	mb, ok := rmb.metricBuilders[metric.MetricName]
	if !ok {
		mb = newMetricBuilder(rmb.rms, metric.MetricName, metric.Unit)
		rmb.metricBuilders[metric.MetricName] = mb
	}
	mb.AddDataPoint(metric)
}

// setAttributes creates a pcommon.Resource from the fields in the resourceMetricsBuilder.
func (rmb *resourceAttributes) setAttributes(resource pcommon.Resource) {
	attributes := resource.Attributes()
	attributes.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attributes.PutStr(conventions.AttributeCloudAccountID, rmb.accountID)
	attributes.PutStr(conventions.AttributeCloudRegion, rmb.region)
	serviceNamespace, serviceName := toServiceAttributes(rmb.namespace)
	if serviceNamespace != "" {
		attributes.PutStr(conventions.AttributeServiceNamespace, serviceNamespace)
	}
	attributes.PutStr(conventions.AttributeServiceName, serviceName)
	attributes.PutStr(attributeAWSCloudWatchMetricStreamName, rmb.metricStreamName)
}

// toServiceAttributes splits the CloudWatch namespace into service namespace/name
// if prepended by AWS/. Otherwise, it returns the CloudWatch namespace as the
// service name with an empty service namespace
func toServiceAttributes(namespace string) (serviceNamespace, serviceName string) {
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
	metric pmetric.Metric
	// seen is the set of added data point keys.
	seen map[dataPointKey]bool
}

// newMetricBuilder creates a metricBuilder with the name and unit.
func newMetricBuilder(rms pmetric.MetricSlice, name, unit string) *metricBuilder {
	m := rms.AppendEmpty()
	m.SetName(name)
	m.SetUnit(unit)
	m.SetEmptySummary()
	return &metricBuilder{
		metric: m,
		seen:   make(map[dataPointKey]bool),
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
		mb.toDataPoint(mb.metric.Summary().DataPoints().AppendEmpty(), metric)
		mb.seen[key] = true
	}
}

// toDataPoint converts a cWMetric into a pdata datapoint and attaches the
// dimensions as attributes.
func (mb *metricBuilder) toDataPoint(dp pmetric.SummaryDataPoint, metric cWMetric) {
	dp.SetCount(uint64(metric.Value.Count))
	dp.SetSum(metric.Value.Sum)
	qv := dp.QuantileValues()
	min := qv.AppendEmpty()
	min.SetQuantile(0)
	min.SetValue(metric.Value.Min)
	max := qv.AppendEmpty()
	max.SetQuantile(1)
	max.SetValue(metric.Value.Max)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(metric.Timestamp)))
	for k, v := range metric.Dimensions {
		dp.Attributes().PutStr(ToSemConvAttributeKey(k), v)
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
