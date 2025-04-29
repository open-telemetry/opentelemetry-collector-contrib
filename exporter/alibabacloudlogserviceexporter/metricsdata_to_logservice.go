// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

import (
	"sort"
	"strconv"
	"strings"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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

func (kv *KeyValues) Sort() {
	sort.Sort(kv)
}

func (kv *KeyValues) Replace(key, value string) {
	key = sanitize(key)
	findIndex := sort.Search(len(kv.keyValues), func(index int) bool {
		return kv.keyValues[index].Key >= key
	})
	if findIndex < len(kv.keyValues) && kv.keyValues[findIndex].Key == key {
		kv.keyValues[findIndex].Value = value
	}
}

func (kv *KeyValues) Append(key, value string) {
	key = sanitize(key)
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
		}
		if newName == nil {
			newName = []byte(name)
		}
		newName[i] = '_'
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
	value float64,
) *sls.Log {
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

func resourceToMetricLabels(labels *KeyValues, resource pcommon.Resource) {
	attrs := resource.Attributes()
	for k, v := range attrs.All() {
		labels.Append(k, v.AsString())
	}
}

func numberMetricsToLogs(name string, data pmetric.NumberDataPointSlice, defaultLabels KeyValues) (logs []*sls.Log) {
	for i := 0; i < data.Len(); i++ {
		dataPoint := data.At(i)
		attributeMap := dataPoint.Attributes()
		labels := defaultLabels.Clone()
		for k, v := range attributeMap.All() {
			labels.Append(k, v.AsString())
		}
		switch dataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			logs = append(logs,
				newMetricLogFromRaw(name,
					labels,
					int64(dataPoint.Timestamp()),
					float64(dataPoint.IntValue()),
				),
			)
		case pmetric.NumberDataPointValueTypeDouble:
			logs = append(logs,
				newMetricLogFromRaw(name,
					labels,
					int64(dataPoint.Timestamp()),
					dataPoint.DoubleValue(),
				),
			)
		}
	}
	return logs
}

func doubleHistogramMetricsToLogs(name string, data pmetric.HistogramDataPointSlice, defaultLabels KeyValues) (logs []*sls.Log) {
	for i := 0; i < data.Len(); i++ {
		dataPoint := data.At(i)
		attributeMap := dataPoint.Attributes()
		labels := defaultLabels.Clone()
		for k, v := range attributeMap.All() {
			labels.Append(k, v.AsString())
		}
		logs = append(logs, newMetricLogFromRaw(name+"_sum",
			labels,
			int64(dataPoint.Timestamp()),
			dataPoint.Sum()))
		logs = append(logs, newMetricLogFromRaw(name+"_count",
			labels,
			int64(dataPoint.Timestamp()),
			float64(dataPoint.Count())))

		bounds := dataPoint.ExplicitBounds()
		boundsStr := make([]string, bounds.Len()+1)
		for i := 0; i < bounds.Len(); i++ {
			boundsStr[i] = strconv.FormatFloat(bounds.At(i), 'g', -1, 64)
		}
		boundsStr[len(boundsStr)-1] = infinityBoundValue

		bucketCount := min(len(boundsStr), dataPoint.BucketCounts().Len())

		bucketLabels := labels.Clone()
		bucketLabels.Append(bucketLabelKey, "")
		bucketLabels.Sort()
		for i := 0; i < bucketCount; i++ {
			bucket := dataPoint.BucketCounts().At(i)
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

func doubleSummaryMetricsToLogs(name string, data pmetric.SummaryDataPointSlice, defaultLabels KeyValues) (logs []*sls.Log) {
	for i := 0; i < data.Len(); i++ {
		dataPoint := data.At(i)
		attributeMap := dataPoint.Attributes()
		labels := defaultLabels.Clone()
		for k, v := range attributeMap.All() {
			labels.Append(k, v.AsString())
		}
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

func metricDataToLogServiceData(md pmetric.Metric, defaultLabels KeyValues) (logs []*sls.Log) {
	//exhaustive:enforce
	switch md.Type() {
	case pmetric.MetricTypeEmpty, pmetric.MetricTypeExponentialHistogram:
		break
	case pmetric.MetricTypeGauge:
		return numberMetricsToLogs(md.Name(), md.Gauge().DataPoints(), defaultLabels)
	case pmetric.MetricTypeSum:
		return numberMetricsToLogs(md.Name(), md.Sum().DataPoints(), defaultLabels)
	case pmetric.MetricTypeHistogram:
		return doubleHistogramMetricsToLogs(md.Name(), md.Histogram().DataPoints(), defaultLabels)
	case pmetric.MetricTypeSummary:
		return doubleSummaryMetricsToLogs(md.Name(), md.Summary().DataPoints(), defaultLabels)
	}
	return logs
}

func metricsDataToLogServiceData(
	_ *zap.Logger,
	md pmetric.Metrics,
) (logs []*sls.Log) {
	resMetrics := md.ResourceMetrics()
	for i := 0; i < resMetrics.Len(); i++ {
		resMetricSlice := resMetrics.At(i)
		var defaultLabels KeyValues
		resourceToMetricLabels(&defaultLabels, resMetricSlice.Resource())
		insMetricSlice := resMetricSlice.ScopeMetrics()
		for j := 0; j < insMetricSlice.Len(); j++ {
			insMetrics := insMetricSlice.At(j)
			// ignore insMetrics.Scope()
			metricSlice := insMetrics.Metrics()
			for k := 0; k < metricSlice.Len(); k++ {
				oneMetric := metricSlice.At(k)
				logs = append(logs, metricDataToLogServiceData(oneMetric, defaultLabels)...)
			}
		}
	}

	return logs
}
