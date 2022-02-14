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

package cwmetricstream // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/unmarshaler/cwmetricstream"
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

type resourceMetricsBuilder struct {
	metricStreamName string
	accountID        string
	region           string
	namespace        string
	metricBuilders   map[string]*metricBuilder
}

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
	metricKey := rmb.toMetricKey(metric)
	mb, ok := rmb.metricBuilders[metricKey]
	if !ok {
		mb = newMetricBuilder(metric.MetricName, metric.Unit)
		rmb.metricBuilders[metricKey] = mb
	}
	mb.AddDataPoint(metric)
}

// Build creates the pdata.ResourceMetrics from the added metrics.
func (rmb *resourceMetricsBuilder) Build() pdata.ResourceMetrics {
	rm := pdata.NewResourceMetrics()
	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	rmb.toResource().CopyTo(rm.Resource())
	for _, mb := range rmb.metricBuilders {
		mb.Build().CopyTo(ilm.Metrics().AppendEmpty())
	}
	return rm
}

// toResource creates a pdata.Resource from the fields in the resourceMetricsBuilder.
// Splits the namespace into service.namespace/service.name if prepended by AWS/.
func (rmb *resourceMetricsBuilder) toResource() pdata.Resource {
	resource := pdata.NewResource()
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
	return resource
}

// toMetricKey creates a key based on the metric name and dimensions to
// keep metrics with different dimensions separate.
func (rmb *resourceMetricsBuilder) toMetricKey(metric cWMetric) string {
	return fmt.Sprintf("%s::%v", metric.MetricName, metric.Dimensions)
}

type metricBuilder struct {
	name       string
	unit       string
	dataPoints pdata.HistogramDataPointSlice
	timestamps map[int64]bool
}

func newMetricBuilder(name, unit string) *metricBuilder {
	return &metricBuilder{
		name:       name,
		unit:       unit,
		dataPoints: pdata.NewHistogramDataPointSlice(),
		timestamps: make(map[int64]bool),
	}
}

// AddDataPoint adds the metric as a datapoint if a metric for that timestamp
// hasn't already been added
func (mb *metricBuilder) AddDataPoint(metric cWMetric) {
	if _, ok := mb.timestamps[metric.Timestamp]; !ok {
		mb.toDataPoint(metric).CopyTo(mb.dataPoints.AppendEmpty())
		mb.timestamps[metric.Timestamp] = true
	}
}

// Build builds the pdata.Metric with the data points that were added
// with AddDataPoint
func (mb *metricBuilder) Build() pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(mb.name)
	metric.SetUnit(mb.unit)
	metric.SetDataType(pdata.MetricDataTypeHistogram)
	metric.Histogram().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	mb.dataPoints.CopyTo(metric.Histogram().DataPoints())
	return metric
}

// toDataPoint converts a cWMetric into a pdata datapoint and attaches the
// dimensions as attributes
func (mb *metricBuilder) toDataPoint(metric cWMetric) pdata.HistogramDataPoint {
	dp := pdata.NewHistogramDataPoint()
	dp.SetCount(uint64(metric.Value.Count))
	dp.SetSum(metric.Value.Sum)
	dp.SetExplicitBounds([]float64{metric.Value.Min, metric.Value.Max})
	// TODO: need to set start timestamp as well
	dp.SetTimestamp(pdata.NewTimestampFromTime(time.UnixMilli(metric.Timestamp)))
	for k, v := range metric.Dimensions {
		dp.Attributes().InsertString(ToSemConvAttributeKey(k), v)
	}
	return dp
}

// ToSemConvAttributeKey maps some common keys to semantic convention attributes
func ToSemConvAttributeKey(key string) string {
	switch key {
	case dimensionInstanceID:
		return conventions.AttributeServiceInstanceID
	default:
		return key
	}
}
