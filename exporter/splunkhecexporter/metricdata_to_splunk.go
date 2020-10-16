// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

const (
	// unknownHostName is the default host name when no hostname label is passed.
	unknownHostName = "unknown"
	// separator for metric values.
	separator = "."
	// splunkMetricValue is the splunk metric value prefix.
	splunkMetricValue = "metric_name"
	// bucketSuffix is the bucket suffix for distribution buckets.
	bucketSuffix = "bucket"
	// quantileSuffix is the quantile suffix for summary quantiles.
	quantileSuffix = "quantile"
	// countSuffix is the count metric value suffix.
	countSuffix = "count"
	// sumOfSquaredDeviation is the sum of squared deviation metric value suffix.
	sumOfSquaredDeviation = "sum_of_squared_deviation"
)

var (
	// infinity bound dimension value is used on all histograms.
	infinityBoundSFxDimValue = float64ToDimValue(math.Inf(1))
)

func metricDataToSplunk(logger *zap.Logger, data pdata.Metrics, config *Config) ([]*splunk.Event, int, error) {
	ocmds := internaldata.MetricsToOC(data)
	numDroppedTimeSeries := 0
	_, numPoints := data.MetricAndDataPointCount()
	splunkMetrics := make([]*splunk.Event, 0, numPoints)
	for _, ocmd := range ocmds {
		var host string
		if ocmd.Resource != nil {
			host = ocmd.Resource.Labels[conventions.AttributeHostHostname]
		}
		if host == "" {
			host = unknownHostName
		}
		for _, metric := range ocmd.Metrics {
			for _, timeSeries := range metric.Timeseries {
				for _, tsPoint := range timeSeries.Points {
					values, err := mapValues(logger, metric, tsPoint.GetValue())
					if err != nil {
						logger.Warn(
							"Timeseries dropped to unexpected metric type",
							zap.Any("metric", metric),
							zap.Any("err", err))
						numDroppedTimeSeries++
						continue
					}
					for key, value := range values {
						if value == nil {
							logger.Warn(
								"Timeseries dropped to unexpected metric type",
								zap.Any("metric", value))
							numDroppedTimeSeries++
							continue
						}
						fields := map[string]interface{}{key: value}
						for k, v := range ocmd.Node.GetAttributes() {
							fields[k] = v
						}
						for k, v := range ocmd.Resource.GetLabels() {
							fields[k] = v
						}
						for i, desc := range metric.MetricDescriptor.GetLabelKeys() {
							fields[desc.Key] = timeSeries.LabelValues[i].Value
						}
						sm := &splunk.Event{
							Time:       timestampToEpochMilliseconds(tsPoint.GetTimestamp()),
							Host:       host,
							Source:     config.Source,
							SourceType: config.SourceType,
							Index:      config.Index,
							Event:      splunk.HecEventMetricType,
							Fields:     fields,
						}
						splunkMetrics = append(splunkMetrics, sm)
					}
				}
			}
		}
	}

	return splunkMetrics, numDroppedTimeSeries, nil
}

func timestampToEpochMilliseconds(ts *timestamppb.Timestamp) float64 {
	if ts == nil {
		return 0
	}
	return float64(ts.GetSeconds()) + math.Round(float64(ts.GetNanos())/1e6)/1e3
}

func mapValues(logger *zap.Logger, metric *metricspb.Metric, value interface{}) (map[string]interface{}, error) {
	metricName := fmt.Sprintf("%s:%s", splunkMetricValue, metric.GetMetricDescriptor().Name)
	switch pv := value.(type) {
	case *metricspb.Point_Int64Value:
		return map[string]interface{}{metricName: pv.Int64Value}, nil
	case *metricspb.Point_DoubleValue:
		return map[string]interface{}{metricName: pv.DoubleValue}, nil
	case *metricspb.Point_DistributionValue:
		return mapDistributionValue(metric, pv.DistributionValue)
	case *metricspb.Point_SummaryValue:
		return mapSummaryValue(metric, pv.SummaryValue)
	default:
		return nil, errors.New("unsupported metric type")
	}
}

func mapDistributionValue(metric *metricspb.Metric, distributionValue *metricspb.DistributionValue) (map[string]interface{}, error) {
	// Translating distribution values per symmetrical recommendations to Prometheus:
	// https://docs.signalfx.com/en/latest/integrations/agent/monitors/prometheus-exporter.html#overview

	// 1. The total count gets converted to a cumulative counter called
	// <basename>_count.
	// 2. The total sum gets converted to a cumulative counter called <basename>.
	values := make(map[string]interface{})
	metricName := fmt.Sprintf("%s:%s", splunkMetricValue, metric.GetMetricDescriptor().Name)
	values[metricName+separator+countSuffix] = distributionValue.Count
	values[metricName] = distributionValue.Sum
	values[metricName+separator+sumOfSquaredDeviation] = distributionValue.SumOfSquaredDeviation

	// 3. Each histogram bucket is converted to a cumulative counter called
	// <basename>_bucket and will include a dimension called upper_bound that
	// specifies the maximum value in that bucket. This metric specifies the
	// number of events with a value that is less than or equal to the upper
	// bound.
	bucketMetricName := metricName + separator + bucketSuffix + separator
	explicitBuckets := distributionValue.BucketOptions.GetExplicit()
	if explicitBuckets == nil {
		return values, fmt.Errorf(
			"unknown bucket options type for metric %q",
			bucketMetricName)
	}
	bounds := explicitBuckets.Bounds
	splunkBounds := make([]*string, len(bounds)+1)
	for i := 0; i < len(bounds); i++ {
		dimValue := float64ToDimValue(bounds[i])
		splunkBounds[i] = &dimValue
	}
	splunkBounds[len(splunkBounds)-1] = &infinityBoundSFxDimValue

	for i, bucket := range distributionValue.Buckets {
		values[fmt.Sprintf("%s%s", bucketMetricName, *splunkBounds[i])] = bucket.Count
	}

	return values, nil
}

func float64ToDimValue(f float64) string {
	// Parameters below are the same used by Prometheus
	// see https://github.com/prometheus/common/blob/b5fe7d854c42dc7842e48d1ca58f60feae09d77b/expfmt/text_create.go#L450
	// SignalFx agent uses a different pattern
	// https://github.com/signalfx/signalfx-agent/blob/5779a3de0c9861fa07316fd11b3c4ff38c0d78f0/internal/monitors/prometheusexporter/conversion.go#L77
	// The important issue here is consistency with the exporter, opting for the
	// more common one used by Prometheus.
	str := strconv.FormatFloat(f, 'g', -1, 64)
	return str
}

func mapSummaryValue(metric *metricspb.Metric, summaryValue *metricspb.SummaryValue) (map[string]interface{}, error) {

	// Translating summary values per symmetrical recommendations to Prometheus:
	// https://docs.signalfx.com/en/latest/integrations/agent/monitors/prometheus-exporter.html#overview

	// 1. The total count gets converted to a cumulative counter called
	// <basename>_count.
	// 2. The total sum gets converted to a cumulative counter called <basename>
	values := make(map[string]interface{})
	metricName := fmt.Sprintf("%s:%s", splunkMetricValue, metric.GetMetricDescriptor().Name)
	values[metricName+separator+countSuffix] = summaryValue.Count.Value
	values[metricName] = summaryValue.Sum.Value

	// 3. Each quantile value is converted to a gauge called <basename>_quantile
	// and will include a dimension called quantile that specifies the quantile.
	percentiles := summaryValue.GetSnapshot().GetPercentileValues()
	if percentiles == nil {
		return values, fmt.Errorf(
			"unknown percentiles values for summary metric %q",
			metricName)
	}
	quantileMetricName := metricName + separator + quantileSuffix + separator
	for _, quantile := range percentiles {
		values[quantileMetricName+float64ToDimValue(quantile.Percentile)] = quantile.Value

	}

	return values, nil
}
