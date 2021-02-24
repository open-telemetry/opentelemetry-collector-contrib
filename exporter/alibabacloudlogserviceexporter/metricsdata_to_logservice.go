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
	"sort"
	"strconv"
	"strings"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/consumer/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"
)

const (
	metricNameKey = "__name__"
	labelsKey     = "__labels__"
	timeNanoKey   = "__time_nano__"
	valueKey      = "__value__"
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

func formatMetricName(name string) string {
	var newName []byte
	for i := 0; i < len(name); i++ {
		b := name[i]
		if (b >= 'a' && b <= 'z') ||
			(b >= 'A' && b <= 'Z') ||
			(b >= '0' && b <= '9') ||
			b == '_' ||
			b == ':' {
			continue
		} else {
			if newName == nil {
				newName = []byte(name)
			}
			newName[i] = '_'
		}
	}
	if newName == nil {
		return name
	}
	return string(newName)
}

func newMetricLogFromRaw(
	name string,
	labels KeyValues,
	nsec int64,
	value float64) *sls.Log {
	labels.Sort()
	return &sls.Log{
		Time: proto.Uint32(uint32(nsec / 1e9)),
		Contents: []*sls.LogContent{
			{
				Key:   proto.String(metricNameKey),
				Value: proto.String(formatMetricName(name)),
			},
			{
				Key:   proto.String(labelsKey),
				Value: proto.String(labels.String()),
			},
			{
				Key:   proto.String(timeNanoKey),
				Value: proto.String(strconv.FormatInt(nsec, 10)),
			},
			{
				Key:   proto.String(valueKey),
				Value: proto.String(strconv.FormatFloat(value, 'g', -1, 64)),
			},
		},
	}
}

func min(l, r int) int {
	if l < r {
		return l
	}
	return r
}

func resourceToMetricLabels(labels *KeyValues, resource pdata.Resource) {
	attrs := resource.Attributes()
	attrs.ForEach(func(k string, v pdata.AttributeValue) {
		labels.keyValues = append(labels.keyValues, KeyValue{
			Key:   k,
			Value: tracetranslator.AttributeValueToString(v, false),
		})
	})
}

func intMetricsToLogs(name string, data pdata.IntDataPointSlice, defaultLabels KeyValues) (logs []*sls.Log) {
	for i := 0; i < data.Len(); i++ {
		dataPoint := data.At(i)
		labelsMap := dataPoint.LabelsMap()
		labels := defaultLabels.Clone()
		labelsMap.ForEach(func(k string, v string) {
			labels.Append(k, v)
		})
		logs = append(logs, newMetricLogFromRaw(name,
			labels,
			int64(dataPoint.Timestamp()),
			float64(dataPoint.Value())))
	}
	return logs
}

func doubleMetricsToLogs(name string, data pdata.DoubleDataPointSlice, defaultLabels KeyValues) (logs []*sls.Log) {
	for i := 0; i < data.Len(); i++ {
		dataPoint := data.At(i)
		labelsMap := dataPoint.LabelsMap()
		labels := defaultLabels.Clone()
		labelsMap.ForEach(func(k string, v string) {
			labels.Append(k, v)
		})
		logs = append(logs, newMetricLogFromRaw(name,
			labels,
			int64(dataPoint.Timestamp()),
			dataPoint.Value()))
	}
	return logs
}

func intHistogramMetricsToLogs(name string, data pdata.IntHistogramDataPointSlice, defaultLabels KeyValues) (logs []*sls.Log) {
	for i := 0; i < data.Len(); i++ {
		dataPoint := data.At(i)
		labelsMap := dataPoint.LabelsMap()
		labels := defaultLabels.Clone()
		labelsMap.ForEach(func(k string, v string) {
			labels.Append(k, v)
		})
		logs = append(logs, newMetricLogFromRaw(name+"_sum",
			labels,
			int64(dataPoint.Timestamp()),
			float64(dataPoint.Sum())))
		logs = append(logs, newMetricLogFromRaw(name+"_count",
			labels,
			int64(dataPoint.Timestamp()),
			float64(dataPoint.Count())))

		bounds := dataPoint.ExplicitBounds()
		boundsStr := make([]string, len(bounds)+1)
		for i := 0; i < len(bounds); i++ {
			boundsStr[i] = strconv.FormatFloat(bounds[i], 'g', -1, 64)
		}
		boundsStr[len(boundsStr)-1] = infinityBoundValue

		bucketCount := min(len(boundsStr), len(dataPoint.BucketCounts()))

		bucketLabels := labels.Clone()
		bucketLabels.Append(bucketLabelKey, "")
		bucketLabels.Sort()
		for i := 0; i < bucketCount; i++ {
			bucket := dataPoint.BucketCounts()[i]
			bucketLabels.Replace(bucketLabelKey, boundsStr[i])

			logs = append(
				logs,
				newMetricLogFromRaw(
					name+"_bucket",
					bucketLabels,
					int64(dataPoint.Timestamp()),
					float64(bucket),
				))
		}

	}
	return logs
}

func doubleHistogramMetricsToLogs(name string, data pdata.DoubleHistogramDataPointSlice, defaultLabels KeyValues) (logs []*sls.Log) {
	for i := 0; i < data.Len(); i++ {
		dataPoint := data.At(i)
		labelsMap := dataPoint.LabelsMap()
		labels := defaultLabels.Clone()
		labelsMap.ForEach(func(k string, v string) {
			labels.Append(k, v)
		})
		logs = append(logs, newMetricLogFromRaw(name+"_sum",
			labels,
			int64(dataPoint.Timestamp()),
			dataPoint.Sum()))
		logs = append(logs, newMetricLogFromRaw(name+"_count",
			labels,
			int64(dataPoint.Timestamp()),
			float64(dataPoint.Count())))

		bounds := dataPoint.ExplicitBounds()
		boundsStr := make([]string, len(bounds)+1)
		for i := 0; i < len(bounds); i++ {
			boundsStr[i] = strconv.FormatFloat(bounds[i], 'g', -1, 64)
		}
		boundsStr[len(boundsStr)-1] = infinityBoundValue

		bucketCount := min(len(boundsStr), len(dataPoint.BucketCounts()))

		bucketLabels := labels.Clone()
		bucketLabels.Append(bucketLabelKey, "")
		bucketLabels.Sort()
		for i := 0; i < bucketCount; i++ {
			bucket := dataPoint.BucketCounts()[i]
			bucketLabels.Replace(bucketLabelKey, boundsStr[i])

			logs = append(
				logs,
				newMetricLogFromRaw(
					name+"_bucket",
					bucketLabels,
					int64(dataPoint.Timestamp()),
					float64(bucket),
				))
		}

	}
	return logs
}

func doubleSummaryMetricsToLogs(name string, data pdata.DoubleSummaryDataPointSlice, defaultLabels KeyValues) (logs []*sls.Log) {
	for i := 0; i < data.Len(); i++ {
		dataPoint := data.At(i)
		labelsMap := dataPoint.LabelsMap()
		labels := defaultLabels.Clone()
		labelsMap.ForEach(func(k string, v string) {
			labels.Append(k, v)
		})
		logs = append(logs, newMetricLogFromRaw(name+"_sum",
			labels,
			int64(dataPoint.Timestamp()),
			dataPoint.Sum()))
		logs = append(logs, newMetricLogFromRaw(name+"_count",
			labels,
			int64(dataPoint.Timestamp()),
			float64(dataPoint.Count())))

		// Adding the "quantile" dimension.
		summaryLabels := labels.Clone()
		summaryLabels.Append(summaryLabelKey, "")
		summaryLabels.Sort()

		values := dataPoint.QuantileValues()
		for i := 0; i < values.Len(); i++ {
			value := values.At(i)
			summaryLabels.Replace(summaryLabelKey, strconv.FormatFloat(value.Quantile(), 'g', -1, 64))
			logs = append(logs, newMetricLogFromRaw(name,
				summaryLabels,
				int64(dataPoint.Timestamp()),
				value.Value()))
		}
	}
	return logs
}

func metricDataToLogServiceData(md pdata.Metric, defaultLabels KeyValues) (logs []*sls.Log) {
	switch md.DataType() {
	case pdata.MetricDataTypeNone:
		break
	case pdata.MetricDataTypeIntGauge:
		return intMetricsToLogs(md.Name(), md.IntGauge().DataPoints(), defaultLabels)
	case pdata.MetricDataTypeDoubleGauge:
		return doubleMetricsToLogs(md.Name(), md.DoubleGauge().DataPoints(), defaultLabels)
	case pdata.MetricDataTypeIntSum:
		return intMetricsToLogs(md.Name(), md.IntSum().DataPoints(), defaultLabels)
	case pdata.MetricDataTypeDoubleSum:
		return doubleMetricsToLogs(md.Name(), md.DoubleSum().DataPoints(), defaultLabels)
	case pdata.MetricDataTypeIntHistogram:
		return intHistogramMetricsToLogs(md.Name(), md.IntHistogram().DataPoints(), defaultLabels)
	case pdata.MetricDataTypeDoubleHistogram:
		return doubleHistogramMetricsToLogs(md.Name(), md.DoubleHistogram().DataPoints(), defaultLabels)
	case pdata.MetricDataTypeDoubleSummary:
		return doubleSummaryMetricsToLogs(md.Name(), md.DoubleSummary().DataPoints(), defaultLabels)
	}
	return logs
}

func metricsDataToLogServiceData(
	_ *zap.Logger,
	md pdata.Metrics,
) (logs []*sls.Log, numDroppedTimeSeries int) {

	resMetrics := md.ResourceMetrics()
	for i := 0; i < resMetrics.Len(); i++ {
		resMetricSlice := resMetrics.At(i)
		var defaultLabels KeyValues
		resourceToMetricLabels(&defaultLabels, resMetricSlice.Resource())
		insMetricSlice := resMetricSlice.InstrumentationLibraryMetrics()
		for j := 0; j < insMetricSlice.Len(); j++ {
			insMetrics := insMetricSlice.At(j)
			// ignore insMetrics.InstrumentationLibrary()
			metricSlice := insMetrics.Metrics()
			for k := 0; k < metricSlice.Len(); k++ {
				oneMetric := metricSlice.At(k)
				logs = append(logs, metricDataToLogServiceData(oneMetric, defaultLabels)...)
			}
		}
	}

	return logs, numDroppedTimeSeries
}
