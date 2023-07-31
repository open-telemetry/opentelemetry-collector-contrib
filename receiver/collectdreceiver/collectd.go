// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

type TargetMetricType string

const (
	GaugeMetricType      = TargetMetricType("gauge")
	CumulativeMetricType = TargetMetricType("cumulative")
)

const (
	collectDMetricDerive  = "derive"
	collectDMetricCounter = "counter"
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

func (cdr *collectDRecord) isEvent() bool {
	return cdr.Time != nil && cdr.Severity != nil && cdr.Message != nil
}

func (cdr *collectDRecord) protoTime() pcommon.Timestamp {
	// Return 1970-01-01 00:00:00 +0000 UTC.
	if cdr.Time == nil {
		return pcommon.NewTimestampFromTime(time.Unix(0, 0))
	}
	ts := time.Unix(0, int64(float64(time.Second)**cdr.Time))
	return pcommon.NewTimestampFromTime(ts)
}

func (cdr *collectDRecord) startTimestamp(metricType TargetMetricType) pcommon.Timestamp {
	if metricType == CumulativeMetricType {
		return pcommon.NewTimestampFromTime(time.Unix(0, int64((*cdr.Time-*cdr.Interval)*float64(time.Second))))
	}
	return pcommon.NewTimestampFromTime(time.Unix(0, 0))
}

func (cdr *collectDRecord) appendToMetrics(scopeMetrics pmetric.ScopeMetrics, defaultLabels map[string]string) error {
	// Ignore if record is an event instead of data point
	if cdr.isEvent() {
		recordEventsReceived()
		return nil
	}

	recordMetricsReceived()
	labels := make(map[string]string, len(defaultLabels))
	for k, v := range defaultLabels {
		labels[k] = v
	}

	for i := range cdr.Dsnames {
		if i < len(cdr.Dstypes) && i < len(cdr.Values) && cdr.Values[i] != nil {
			dsType, dsName, val := cdr.Dstypes[i], cdr.Dsnames[i], cdr.Values[i]
			metricName, usedDsName := cdr.getReasonableMetricName(i, labels)

			addIfNotNullOrEmpty(labels, "plugin", cdr.Plugin)
			parseAndAddLabels(labels, cdr.PluginInstance, cdr.Host)
			if !usedDsName {
				addIfNotNullOrEmpty(labels, "dsname", dsName)
			}

			metric, err := cdr.newMetric(metricName, dsType, val, labels)
			if err != nil {
				return fmt.Errorf("error processing metric %s: %w", sanitize.String(metricName), err)
			}
			newMetric := scopeMetrics.Metrics().AppendEmpty()
			metric.MoveTo(newMetric)
		}
	}
	return nil
}

// Create new metric, get labels, then setting attribute and metric info
func (cdr *collectDRecord) newMetric(name string, dsType *string, val *json.Number, labels map[string]string) (pmetric.Metric, error) {
	attributes := setAttributes(labels)
	metric, err := cdr.setMetric(name, dsType, val, attributes)
	if err != nil {
		return pmetric.Metric{}, fmt.Errorf("error processing metric %s: %w", name, err)
	}
	return metric, nil
}

func setAttributes(labels map[string]string) pcommon.Map {
	attributes := pcommon.NewMap()
	for k, v := range labels {
		attributes.PutStr(k, v)
	}
	return attributes
}

// Set new metric info with name, datapoint, time, attributes
func (cdr *collectDRecord) setMetric(name string, dsType *string, val *json.Number, atr pcommon.Map) (pmetric.Metric, error) {

	typ := ""
	metric := pmetric.NewMetric()

	if dsType != nil {
		typ = *dsType
	}

	metric.SetName(name)
	dataPoint := setDataPoint(typ, metric)
	dataPoint.SetTimestamp(cdr.protoTime())
	atr.CopyTo(dataPoint.Attributes())

	if pointVal, err := val.Int64(); err == nil {
		dataPoint.SetIntValue(pointVal)
	} else if pointVal, err := val.Float64(); err == nil {
		dataPoint.SetDoubleValue(pointVal)
	} else {
		return pmetric.Metric{}, fmt.Errorf("value could not be decoded: %w", err)
	}
	return metric, nil
}

// check type to decide metric type and return data point
func setDataPoint(typ string, metric pmetric.Metric) pmetric.NumberDataPoint {
	var dataPoint pmetric.NumberDataPoint
	switch typ {
	case collectDMetricCounter, collectDMetricDerive:
		sum := metric.SetEmptySum()
		sum.SetIsMonotonic(true)
		dataPoint = sum.DataPoints().AppendEmpty()
	default:
		dataPoint = metric.SetEmptyGauge().DataPoints().AppendEmpty()
	}
	return dataPoint
}

// getReasonableMetricName creates metrics names by joining them (if non empty) type.typeinstance
// if there are more than one dsname append .dsname for the particular uint. if there's only one it
// becomes a dimension.
func (cdr *collectDRecord) getReasonableMetricName(index int, attrs map[string]string) (string, bool) {
	usedDsName := false
	capacity := 0
	if cdr.TypeS != nil {
		capacity += len(*cdr.TypeS)
	}
	if cdr.TypeInstance != nil {
		capacity += len(*cdr.TypeInstance)
	}
	parts := make([]byte, 0, capacity)

	if !isNilOrEmpty(cdr.TypeS) {
		parts = append(parts, *cdr.TypeS...)
	}
	parts = cdr.pointTypeInstance(attrs, parts)
	if cdr.Dsnames != nil && !isNilOrEmpty(cdr.Dsnames[index]) && len(cdr.Dsnames) > 1 {
		if len(parts) > 0 {
			parts = append(parts, '.')
		}
		parts = append(parts, *cdr.Dsnames[index]...)
		usedDsName = true
	}
	return string(parts), usedDsName
}

// pointTypeInstance extracts information from the TypeInstance field and appends to the metric name when possible.
func (cdr *collectDRecord) pointTypeInstance(attrs map[string]string, parts []byte) []byte {
	if isNilOrEmpty(cdr.TypeInstance) {
		return parts
	}

	instanceName, extractedAttrs := LabelsFromName(cdr.TypeInstance)
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
