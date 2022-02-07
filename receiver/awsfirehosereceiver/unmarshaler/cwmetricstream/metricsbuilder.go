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
	"time"

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

const (
	attributeAWSCloudWatchNamespace        = "aws.cloudwatch.namespace"
	attributeAWSCloudWatchMetricStreamName = "aws.cloudwatch.metric_stream_name"
	attributeAWSCloudWatchDimension        = "aws.cloudwatch.dimension"
)

type namespacedMetricsBuilder struct {
	resource       pdata.Resource
	metricBuilders map[string]*metricBuilder
}

func newNamespacedMetricsBuilder() *namespacedMetricsBuilder {
	return &namespacedMetricsBuilder{
		resource:       pdata.NewResource(),
		metricBuilders: make(map[string]*metricBuilder),
	}
}

func (nmb *namespacedMetricsBuilder) AddMetric(metric cWMetric) {
	if nmb.resource.Attributes().Len() == 0 {
		nmb.resource = nmb.buildResource(metric)
	}
	key := nmb.toKey(metric)
	mb, ok := nmb.metricBuilders[key]
	if !ok {
		mb = newMetricBuilder(metric.MetricName, metric.Unit, key)
		nmb.metricBuilders[key] = mb
	}
	mb.AddDataPoint(metric)
}

func (nmb *namespacedMetricsBuilder) Build() pdata.ResourceMetrics {
	rm := pdata.NewResourceMetrics()
	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	nmb.resource.CopyTo(rm.Resource())
	for _, mb := range nmb.metricBuilders {
		mb.metric.CopyTo(ilm.Metrics().AppendEmpty())
	}
	return rm
}

func (nmb *namespacedMetricsBuilder) buildResource(metric cWMetric) pdata.Resource {
	resource := pdata.NewResource()
	attributes := resource.Attributes()
	attributes.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attributes.InsertString(conventions.AttributeCloudAccountID, metric.AccountId)
	attributes.InsertString(conventions.AttributeCloudRegion, metric.Region)
	attributes.InsertString(attributeAWSCloudWatchNamespace, metric.Namespace)
	attributes.InsertString(attributeAWSCloudWatchMetricStreamName, metric.MetricStreamName)
	return resource
}

func (nmb *namespacedMetricsBuilder) toKey(metric cWMetric) string {
	key := metric.MetricName
	for k, v := range metric.Dimensions {
		key += fmt.Sprintf("::%s=%s", k, v)
	}
	return key
}

type metricBuilder struct {
	metric     pdata.Metric
	timestamps map[int64]bool
}

func newMetricBuilder(name, unit, key string) *metricBuilder {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetUnit(unit)
	metric.SetDataType(pdata.MetricDataTypeGauge)
	metric.SetDescription(key)
	return &metricBuilder{
		metric:     metric,
		timestamps: make(map[int64]bool),
	}
}

func (mb *metricBuilder) AddDataPoint(metric cWMetric) {
	if mb.metric.Name() == "" {
		mb.metric.SetName(metric.MetricName)
		mb.metric.SetUnit(metric.Unit)
		mb.metric.SetDataType(pdata.MetricDataTypeGauge)
	} else if mb.metric.Name() != metric.MetricName {
		// skipping invalid metric
		return
	}

	if _, ok := mb.timestamps[metric.Timestamp]; !ok {
		mb.buildDataPoint(metric).CopyTo(mb.metric.Gauge().DataPoints().AppendEmpty())
		mb.timestamps[metric.Timestamp] = true
	}
}

func (mb *metricBuilder) buildDataPoint(metric cWMetric) pdata.NumberDataPoint {
	dp := pdata.NewNumberDataPoint()
	dp.SetDoubleVal(metric.Value.Count)
	dp.SetTimestamp(pdata.NewTimestampFromTime(time.UnixMilli(metric.Timestamp)))
	for k, v := range metric.Dimensions {
		dimensionKey := fmt.Sprintf("%s.%s", attributeAWSCloudWatchDimension, k)
		dp.Attributes().InsertString(dimensionKey, v)
	}
	return dp
}
