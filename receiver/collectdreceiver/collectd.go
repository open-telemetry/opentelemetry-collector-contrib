// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

const (
	collectDMetricDerive   = "derive"
	collectDMetricGauge    = "gauge"
	collectDMetricCounter  = "counter"
	collectDMetricAbsolute = "absolute"
)

type collectDRecord struct {
	Dsnames        []*string              `json:"dsnames"`
	Dstypes        []*string              `json:"dstypes"`
	Host           *string                `json:"host"`
	Interval       *float64               `json:"interval"`
	Plugin         *string                `json:"plugin"`
	PluginInstance *string                `json:"plugin_instance"`
	Time           *float64               `json:"time"`
	TypeS          *string                `json:"type"`
	TypeInstance   *string                `json:"type_instance"`
	Values         []*json.Number         `json:"values"`
	Message        *string                `json:"message"`
	Meta           map[string]interface{} `json:"meta"`
	Severity       *string                `json:"severity"`
}

func (r *collectDRecord) isEvent() bool {
	return r.Time != nil && r.Severity != nil && r.Message != nil
}

func (r *collectDRecord) protoTime() *timestamppb.Timestamp {
	if r.Time == nil {
		return nil
	}
	ts := time.Unix(0, int64(float64(time.Second)**r.Time))
	return timestamppb.New(ts)
}

func (r *collectDRecord) startTimestamp(mdType metricspb.MetricDescriptor_Type) *timestamppb.Timestamp {
	if mdType == metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION || mdType == metricspb.MetricDescriptor_CUMULATIVE_DOUBLE || mdType == metricspb.MetricDescriptor_CUMULATIVE_INT64 {
		return timestamppb.New(time.Unix(0, int64((*r.Time-*r.Interval)*float64(time.Second))))
	}
	return nil
}

func (r *collectDRecord) appendToMetrics(metrics []*metricspb.Metric, defaultLabels map[string]string) ([]*metricspb.Metric, error) {
	// Ignore if record is an event instead of data point
	if r.isEvent() {
		recordEventsReceived()
		return metrics, nil

	}

	recordMetricsReceived()
	labels := make(map[string]string, len(defaultLabels))
	for k, v := range defaultLabels {
		labels[k] = v
	}

	for i := range r.Dsnames {
		if i < len(r.Dstypes) && i < len(r.Values) && r.Values[i] != nil {
			dsType, dsName, val := r.Dstypes[i], r.Dsnames[i], r.Values[i]
			metricName, usedDsName := r.getReasonableMetricName(i, labels)

			addIfNotNullOrEmpty(labels, "plugin", r.Plugin)
			parseAndAddLabels(labels, r.PluginInstance, r.Host)
			if !usedDsName {
				addIfNotNullOrEmpty(labels, "dsname", dsName)
			}

			metric, err := r.newMetric(metricName, dsType, val, labels)
			if err != nil {
				return metrics, fmt.Errorf("error processing metric %s: %w", sanitize.String(metricName), err)
			}
			metrics = append(metrics, metric)

		}
	}
	return metrics, nil
}

func (r *collectDRecord) newMetric(name string, dsType *string, val *json.Number, labels map[string]string) (*metricspb.Metric, error) {
	metric := &metricspb.Metric{}
	point, isDouble, err := r.newPoint(val)
	if err != nil {
		return metric, fmt.Errorf("error processing metric %s: %w", name, err)
	}

	lKeys, lValues := labelKeysAndValues(labels)
	metricType := r.metricType(dsType, isDouble)
	metric.MetricDescriptor = &metricspb.MetricDescriptor{
		Name:      name,
		Type:      metricType,
		LabelKeys: lKeys,
	}
	metric.Timeseries = []*metricspb.TimeSeries{
		{
			StartTimestamp: r.startTimestamp(metricType),
			LabelValues:    lValues,
			Points:         []*metricspb.Point{point},
		},
	}

	return metric, nil
}

func (r *collectDRecord) metricType(dsType *string, isDouble bool) metricspb.MetricDescriptor_Type {
	val := ""
	if dsType != nil {
		val = *dsType
	}

	switch val {
	case collectDMetricCounter, collectDMetricDerive:
		return metricCumulative(isDouble)

	// Prometheus collectd exporter just ignores it. We use gauge for it as it seems the
	// closes type. https://github.com/prometheus/collectd_exporter/blob/master/main.go#L109-L129
	case collectDMetricGauge, collectDMetricAbsolute:
		return metricGauge(isDouble)
	}
	return metricGauge(isDouble)
}

func (r *collectDRecord) newPoint(val *json.Number) (*metricspb.Point, bool, error) {
	p := &metricspb.Point{
		Timestamp: r.protoTime(),
	}

	isDouble := true
	if v, err := val.Int64(); err == nil {
		isDouble = false
		p.Value = &metricspb.Point_Int64Value{Int64Value: v}
	} else {
		v, err := val.Float64()
		if err != nil {
			return nil, isDouble, fmt.Errorf("value could not be decoded: %w", err)
		}
		p.Value = &metricspb.Point_DoubleValue{DoubleValue: v}
	}
	return p, isDouble, nil
}

// getReasonableMetricName creates metrics names by joining them (if non empty) type.typeinstance
// if there are more than one dsname append .dsname for the particular uint. if there's only one it
// becomes a dimension.
func (r *collectDRecord) getReasonableMetricName(index int, attrs map[string]string) (string, bool) {
	usedDsName := false
	capacity := 0
	if r.TypeS != nil {
		capacity += len(*r.TypeS)
	}
	if r.TypeInstance != nil {
		capacity += len(*r.TypeInstance)
	}
	parts := make([]byte, 0, capacity)

	if !isNilOrEmpty(r.TypeS) {
		parts = append(parts, *r.TypeS...)
	}
	parts = r.pointTypeInstance(attrs, parts)
	if r.Dsnames != nil && !isNilOrEmpty(r.Dsnames[index]) && len(r.Dsnames) > 1 {
		if len(parts) > 0 {
			parts = append(parts, '.')
		}
		parts = append(parts, *r.Dsnames[index]...)
		usedDsName = true
	}
	return string(parts), usedDsName
}

// pointTypeInstance extracts information from the TypeInstance field and appends to the metric name when possible.
func (r *collectDRecord) pointTypeInstance(attrs map[string]string, parts []byte) []byte {
	if isNilOrEmpty(r.TypeInstance) {
		return parts
	}

	instanceName, extractedAttrs := LabelsFromName(r.TypeInstance)
	if instanceName != "" {
		if len(parts) > 0 {
			parts = append(parts, '.')
		}
		parts = append(parts, instanceName...)
	}
	for k, v := range extractedAttrs {
		if _, exists := attrs[k]; !exists {
			val := v
			addIfNotNullOrEmpty(attrs, k, &val)
		}
	}
	return parts
}

// LabelsFromName tries to pull out dimensions out of name in the format
// "name[k=v,f=x]-more_name".
// For the example above it would return "name-more_name" and extract dimensions
// (k,v) and (f,x).
// If something unexpected is encountered it returns the original metric name.
//
// The code tries to avoid allocation by using local slices and avoiding calls
// to functions like strings.Slice.
func LabelsFromName(val *string) (metricName string, labels map[string]string) {
	metricName = *val
	index := strings.Index(*val, "[")
	if index > -1 {
		left := (*val)[:index]
		rest := (*val)[index+1:]
		index = strings.Index(rest, "]")
		if index > -1 {
			working := make(map[string]string)
			dimensions := rest[:index]
			rest = rest[index+1:]
			cindex := strings.Index(dimensions, ",")
			prev := 0
			for {
				if cindex < prev {
					cindex = len(dimensions)
				}
				piece := dimensions[prev:cindex]
				tindex := strings.Index(piece, "=")
				if tindex == -1 || strings.Contains(piece[tindex+1:], "=") {
					return
				}
				working[piece[:tindex]] = piece[tindex+1:]
				if cindex == len(dimensions) {
					break
				}
				prev = cindex + 1
				cindex = strings.Index(dimensions[prev:], ",") + prev
			}
			labels = working
			metricName = left + rest
		}
	}
	return
}

func isNilOrEmpty(str *string) bool {
	return str == nil || *str == ""
}

func addIfNotNullOrEmpty(m map[string]string, key string, val *string) {
	if val != nil && *val != "" {
		m[key] = *val
	}
}

func parseAndAddLabels(labels map[string]string, pluginInstance *string, host *string) {
	parseNameForLabels(labels, "plugin_instance", pluginInstance)
	parseNameForLabels(labels, "host", host)
}

func parseNameForLabels(labels map[string]string, key string, val *string) {
	instanceName, toAddDims := LabelsFromName(val)

	for k, v := range toAddDims {
		if _, exists := labels[k]; !exists {
			val := v
			addIfNotNullOrEmpty(labels, k, &val)
		}
	}
	addIfNotNullOrEmpty(labels, key, &instanceName)
}

func labelKeysAndValues(labels map[string]string) ([]*metricspb.LabelKey, []*metricspb.LabelValue) {
	keys := make([]*metricspb.LabelKey, len(labels))
	values := make([]*metricspb.LabelValue, len(labels))
	i := 0
	for k, v := range labels {
		keys[i] = &metricspb.LabelKey{Key: k}
		values[i] = &metricspb.LabelValue{Value: v, HasValue: true}
		i++
	}
	return keys, values
}

func metricCumulative(isDouble bool) metricspb.MetricDescriptor_Type {
	if isDouble {
		return metricspb.MetricDescriptor_CUMULATIVE_DOUBLE
	}
	return metricspb.MetricDescriptor_CUMULATIVE_INT64
}

func metricGauge(isDouble bool) metricspb.MetricDescriptor_Type {
	if isDouble {
		return metricspb.MetricDescriptor_GAUGE_DOUBLE
	}
	return metricspb.MetricDescriptor_GAUGE_INT64
}
