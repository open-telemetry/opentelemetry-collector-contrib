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
	dimensionInstanceId                    = "InstanceId"
)

var (
	semConvMapping = map[string]string{
		dimensionInstanceId: conventions.AttributeServiceInstanceID,
	}
)

type resourceMetricsBuilder struct {
	resource       pdata.Resource
	metricBuilders map[string]*metricBuilder
}

func newResourceMetricsBuilder() *resourceMetricsBuilder {
	return &resourceMetricsBuilder{
		resource:       pdata.NewResource(),
		metricBuilders: make(map[string]*metricBuilder),
	}
}

func (rmb *resourceMetricsBuilder) AddMetric(metric cWMetric) {
	if rmb.resource.Attributes().Len() == 0 {
		rmb.resource = rmb.toResource(metric)
	}
	metricKey := rmb.toMetricKey(metric)
	mb, ok := rmb.metricBuilders[metricKey]
	if !ok {
		mb = newMetricBuilder(metric.MetricName, metric.Unit)
		rmb.metricBuilders[metricKey] = mb
	}
	mb.AddDataPoint(metric)
}

func (rmb *resourceMetricsBuilder) Build() pdata.ResourceMetrics {
	rm := pdata.NewResourceMetrics()
	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	rmb.resource.CopyTo(rm.Resource())
	for _, mb := range rmb.metricBuilders {
		mb.Build().CopyTo(ilm.Metrics().AppendEmpty())
	}
	return rm
}

func (rmb *resourceMetricsBuilder) toResource(metric cWMetric) pdata.Resource {
	resource := pdata.NewResource()
	attributes := resource.Attributes()
	attributes.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attributes.InsertString(conventions.AttributeCloudAccountID, metric.AccountId)
	attributes.InsertString(conventions.AttributeCloudRegion, metric.Region)
	// split namespace into service namespace/name (e.g. namespace: AWS/,
	splitNamespace := strings.SplitN(metric.Namespace, "/", 2)
	if len(splitNamespace) == 2 && strings.EqualFold(splitNamespace[0], conventions.AttributeCloudProviderAWS) {
		attributes.InsertString(conventions.AttributeServiceNamespace, splitNamespace[0])
		attributes.InsertString(conventions.AttributeServiceName, splitNamespace[1])
	} else {
		attributes.InsertString(conventions.AttributeServiceName, metric.Namespace)
	}
	attributes.InsertString(attributeAWSCloudWatchMetricStreamName, metric.MetricStreamName)
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

func (mb *metricBuilder) AddDataPoint(metric cWMetric) {
	if _, ok := mb.timestamps[metric.Timestamp]; !ok {
		mb.toDataPoint(metric).CopyTo(mb.dataPoints.AppendEmpty())
		mb.timestamps[metric.Timestamp] = true
	}
}

func (mb *metricBuilder) Build() pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(mb.name)
	metric.SetUnit(mb.unit)
	metric.SetDataType(pdata.MetricDataTypeHistogram)
	metric.Histogram().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	mb.dataPoints.CopyTo(metric.Histogram().DataPoints())
	return metric
}

func (mb *metricBuilder) toDataPoint(metric cWMetric) pdata.HistogramDataPoint {
	dp := pdata.NewHistogramDataPoint()
	dp.SetCount(uint64(metric.Value.Count))
	dp.SetSum(metric.Value.Sum)
	dp.SetExplicitBounds([]float64{metric.Value.Min, metric.Value.Max})
	// need to set start timestamp as well
	dp.SetTimestamp(pdata.NewTimestampFromTime(time.UnixMilli(metric.Timestamp)))
	for k, v := range metric.Dimensions {
		dp.Attributes().InsertString(mb.toSemConvKey(k), v)
	}
	return dp
}

// toCommonKey map some common keys to semantic convention attributes
func (mb *metricBuilder) toSemConvKey(key string) string {
	if sc, ok := semConvMapping[key]; ok {
		return sc
	}
	return key
}
