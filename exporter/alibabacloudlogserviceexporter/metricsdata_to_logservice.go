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

package alibabacloudlogserviceexporter

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

const (
	hostnameKey     = "hostname"
	pidKey          = "pid"
	resourceTypeKey = "resource_type"
	metricNameKey   = "__name__"
	labelsKey       = "__labels__"
	timeNanoKey     = "__time_nano__"
	valueKey        = "__value__"
	// same with : https://github.com/prometheus/common/blob/b5fe7d854c42dc7842e48d1ca58f60feae09d77b/expfmt/text_create.go#L445
	infinityBoundValue = "+Inf"
	bucketLabelKey     = "le"
	summaryLabelKey    = "quantile"
)

type KeyValue struct {
	Key   string
	Value string
}

type KeyValues struct {
	keyValues []KeyValue
}

func (kv *KeyValues) Len() int { return len(kv.keyValues) }
func (kv *KeyValues) Swap(i, j int) {
	kv.keyValues[i], kv.keyValues[j] = kv.keyValues[j], kv.keyValues[i]
}
func (kv *KeyValues) Less(i, j int) bool { return kv.keyValues[i].Key < kv.keyValues[j].Key }
func (kv *KeyValues) Sort()              { sort.Sort(kv) }

func (kv *KeyValues) Replace(key, value string) {
	findIndex := sort.Search(len(kv.keyValues), func(index int) bool {
		return kv.keyValues[index].Key >= key
	})
	if findIndex < len(kv.keyValues) && kv.keyValues[findIndex].Key == key {
		kv.keyValues[findIndex].Value = value
	}
}

func (kv *KeyValues) AppendMap(mapVal map[string]string) {
	for key, value := range mapVal {
		kv.keyValues = append(kv.keyValues, KeyValue{
			Key:   key,
			Value: value,
		})
	}
}

func (kv *KeyValues) Append(key, value string) {
	kv.keyValues = append(kv.keyValues, KeyValue{
		key,
		value,
	})
}

func (kv *KeyValues) Clone() KeyValues {
	var newKeyValues KeyValues
	newKeyValues.keyValues = make([]KeyValue, len(kv.keyValues))
	copy(newKeyValues.keyValues, kv.keyValues)
	return newKeyValues
}

func (kv *KeyValues) String() string {
	var builder strings.Builder
	kv.labelToStringBuilder(&builder)
	return builder.String()
}

func (kv *KeyValues) labelToStringBuilder(sb *strings.Builder) {
	for index, label := range kv.keyValues {
		sb.WriteString(label.Key)
		sb.WriteString("#$#")
		sb.WriteString(label.Value)
		if index != len(kv.keyValues)-1 {
			sb.WriteByte('|')
		}
	}
}

func newMetricLog(nsec int64,
	nameContent *sls.LogContent,
	labelContent *sls.LogContent,
	timeNanoContent *sls.LogContent,
	valueContent *sls.LogContent) *sls.Log {
	return &sls.Log{
		Time:     proto.Uint32(uint32(nsec / 1e9)),
		Contents: []*sls.LogContent{nameContent, labelContent, timeNanoContent, valueContent},
	}
}

func min(l, r int) int {
	if l < r {
		return l
	}
	return r
}

func appendDistributionValues(
	logs []*sls.Log,
	labels KeyValues,
	nsec int64,
	nameContent *sls.LogContent,
	labelContent *sls.LogContent,
	timeNanoContent *sls.LogContent,
	distributionValue *metricspb.DistributionValue,
) ([]*sls.Log, error) {

	// Translating distribution values per symmetrical recommendations to Prometheus:

	// 1. The total count gets converted to a cumulative counter called
	// <basename>_count.
	// 2. The total sum gets converted to a cumulative counter called <basename>_sum
	count := distributionValue.GetCount()
	sum := distributionValue.GetSum()
	logs = appendTotalAndSum(
		logs,
		nsec,
		nameContent,
		labelContent,
		timeNanoContent,
		count,
		sum)

	// 3. Each histogram bucket is converted to a cumulative counter called
	// <basename>_bucket and will include a dimension called upper_bound that
	// specifies the maximum value in that bucket. This metric specifies the
	// number of events with a value that is less than or equal to the upper
	// bound.
	metricNameContent := &sls.LogContent{
		Key:   proto.String(metricNameKey),
		Value: proto.String(nameContent.GetValue() + "_bucket"),
	}
	explicitBuckets := distributionValue.BucketOptions.GetExplicit()
	if explicitBuckets == nil {
		return logs, fmt.Errorf("unknown bucket options type for metric %s", nameContent.GetValue())
	}
	bounds := explicitBuckets.Bounds
	boundsStr := make([]string, len(bounds)+1)
	for i := 0; i < len(bounds); i++ {
		boundsStr[i] = strconv.FormatFloat(bounds[i], 'g', -1, 64)
	}
	boundsStr[len(boundsStr)-1] = infinityBoundValue

	bucketCount := min(len(boundsStr), len(distributionValue.Buckets))

	bucketLabels := labels.Clone()
	bucketLabels.Append(bucketLabelKey, "")
	bucketLabels.Sort()
	for i := 0; i < bucketCount; i++ {
		bucket := distributionValue.Buckets[i]
		bucketLabels.Replace(bucketLabelKey, boundsStr[i])

		logs = append(
			logs,
			newMetricLog(nsec,
				metricNameContent,
				&sls.LogContent{
					Key:   proto.String(labelsKey),
					Value: proto.String(bucketLabels.String()),
				},
				timeNanoContent,
				&sls.LogContent{
					Key:   proto.String(valueKey),
					Value: proto.String(strconv.FormatInt(bucket.Count, 10)),
				},
			))
	}
	return logs, nil
}

func appendSummaryValues(
	logs []*sls.Log,
	labels KeyValues,
	nsec int64,
	nameContent *sls.LogContent,
	labelContent *sls.LogContent,
	timeNanoContent *sls.LogContent,
	summaryValue *metricspb.SummaryValue,
) ([]*sls.Log, error) {

	// Translating summary values per symmetrical recommendations to Prometheus:

	// 1. The total count gets converted to a cumulative counter called
	// <basename>_count.
	// 2. The total sum gets converted to a cumulative counter called <basename>_sum
	count := summaryValue.GetCount().GetValue()
	sum := summaryValue.GetSum().GetValue()
	logs = appendTotalAndSum(
		logs,
		nsec,
		nameContent,
		labelContent,
		timeNanoContent,
		count,
		sum)

	// 3. Each quantile value is converted to a gauge called <basename>
	// and will include a dimension called quantile that specifies the quantile.
	percentiles := summaryValue.GetSnapshot().GetPercentileValues()
	if percentiles == nil {
		return logs, fmt.Errorf(
			"unknown percentiles values for summary metric %q",
			nameContent.GetValue())
	}
	// Adding the "quantile" dimension.
	bucketLabels := labels.Clone()
	bucketLabels.Append(summaryLabelKey, "")
	bucketLabels.Sort()

	for _, quantile := range percentiles {

		bucketLabels.Replace(summaryLabelKey, strconv.FormatFloat(quantile.Percentile, 'g', -1, 64))

		logs = append(
			logs,
			newMetricLog(nsec,
				nameContent,
				&sls.LogContent{
					Key:   proto.String(labelsKey),
					Value: proto.String(bucketLabels.String()),
				},
				timeNanoContent,
				&sls.LogContent{
					Key:   proto.String(valueKey),
					Value: proto.String(strconv.FormatFloat(quantile.Value, 'g', -1, 64)),
				},
			))
	}

	return logs, nil
}

func appendTotalAndSum(
	logs []*sls.Log,
	nsec int64,
	nameContent *sls.LogContent,
	labelContent *sls.LogContent,
	timeNanoContent *sls.LogContent,
	count int64,
	sum float64,
) []*sls.Log {

	logs = append(
		logs,
		newMetricLog(
			nsec,
			&sls.LogContent{
				Key:   proto.String(metricNameKey),
				Value: proto.String(nameContent.GetValue() + "_count"),
			},
			labelContent,
			timeNanoContent,
			&sls.LogContent{
				Key:   proto.String(valueKey),
				Value: proto.String(strconv.FormatInt(count, 10)),
			}),
		newMetricLog(
			nsec,
			&sls.LogContent{
				Key:   proto.String(metricNameKey),
				Value: proto.String(nameContent.GetValue() + "_sum"),
			},
			labelContent,
			timeNanoContent,
			&sls.LogContent{
				Key:   proto.String(valueKey),
				Value: proto.String(strconv.FormatFloat(sum, 'g', -1, 64)),
			}))

	return logs
}

func metricsDataToLogServiceData(
	logger *zap.Logger,
	md consumerdata.MetricsData,
) (logs []*sls.Log, numDroppedTimeSeries int) {

	var defaultLabels KeyValues
	var err error
	// Labels from Node and Resource.
	// TODO: Options to add lib, service name, etc as dimensions?
	//  Q.: what about resource type?

	if hostname := md.Node.GetIdentifier().GetHostName(); hostname != "" {
		defaultLabels.Append(hostnameKey, hostname)
	}
	if pid := int(md.Node.GetIdentifier().GetPid()); pid != 0 {
		defaultLabels.Append(pidKey, strconv.Itoa(pid))
	}
	defaultLabels.AppendMap(md.Node.GetAttributes())

	if resType := md.Resource.GetType(); resType != "" {
		defaultLabels.Append(resourceTypeKey, resType)
	}
	defaultLabels.AppendMap(md.Resource.GetLabels())

	for _, metric := range md.Metrics {
		if metric == nil || metric.MetricDescriptor == nil {
			logger.Warn("Received nil metrics data or nil descriptor for metrics")
			numDroppedTimeSeries += len(metric.GetTimeseries())
			continue
		}

		// Build the fixed parts for this metrics from the descriptor.
		descriptor := metric.MetricDescriptor
		metricNameContent := &sls.LogContent{
			Key:   proto.String(metricNameKey),
			Value: proto.String(descriptor.Name),
		}

		for _, series := range metric.Timeseries {
			labels := defaultLabels.Clone()
			labelCount := min(len(descriptor.LabelKeys), len(series.LabelValues))
			for i := 0; i < labelCount; i++ {
				labels.Append(descriptor.LabelKeys[i].GetKey(), series.LabelValues[i].GetValue())
			}
			labels.Sort()

			labelsContent := &sls.LogContent{
				Key:   proto.String(labelsKey),
				Value: proto.String(labels.String()),
			}

			for _, dp := range series.Points {

				var nsec int64
				if dp.Timestamp != nil {
					nsec = dp.Timestamp.Seconds*1e9 + int64(dp.Timestamp.Nanos)
				} else {
					numDroppedTimeSeries++
					continue
				}

				timeContent := &sls.LogContent{
					Key:   proto.String(timeNanoKey),
					Value: proto.String(strconv.FormatInt(nsec, 10)),
				}

				switch pv := dp.Value.(type) {
				case *metricspb.Point_Int64Value:
					valueContent := &sls.LogContent{
						Key:   proto.String(valueKey),
						Value: proto.String(strconv.FormatInt(pv.Int64Value, 10)),
					}
					logs = append(logs, newMetricLog(nsec, metricNameContent, labelsContent, timeContent, valueContent))
				case *metricspb.Point_DoubleValue:
					valueContent := &sls.LogContent{
						Key:   proto.String(valueKey),
						Value: proto.String(strconv.FormatFloat(pv.DoubleValue, 'g', -1, 64)),
					}
					logs = append(logs, newMetricLog(nsec, metricNameContent, labelsContent, timeContent, valueContent))

				case *metricspb.Point_DistributionValue:
					logs, err = appendDistributionValues(
						logs,
						labels,
						nsec,
						metricNameContent,
						labelsContent,
						timeContent,
						pv.DistributionValue)
					if err != nil {
						numDroppedTimeSeries++
						logger.Warn(
							"Timeseries for distribution metric dropped",
							zap.Error(err),
							zap.String("Metric", descriptor.Name))
					}
				case *metricspb.Point_SummaryValue:
					logs, err = appendSummaryValues(
						logs,
						labels,
						nsec,
						metricNameContent,
						labelsContent,
						timeContent,
						pv.SummaryValue)
					if err != nil {
						numDroppedTimeSeries++
						logger.Warn(
							"Timeseries for summary metric dropped",
							zap.Error(err),
							zap.String("Metric", descriptor.Name))
					}
				default:
					numDroppedTimeSeries++
					logger.Warn(
						"Timeseries dropped to unexpected metric type",
						zap.String("Metric", descriptor.Name))
				}

			}
		}
	}

	return logs, numDroppedTimeSeries
}
