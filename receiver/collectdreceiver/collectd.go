// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/collectd"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

type collectDRecord struct {
	Dsnames        []*string      `json:"dsnames"`
	Dstypes        []*string      `json:"dstypes"`
	Host           *string        `json:"host"`
	Interval       *float64       `json:"interval"`
	Plugin         *string        `json:"plugin"`
	PluginInstance *string        `json:"plugin_instance"`
	Time           *float64       `json:"time"`
	TypeS          *string        `json:"type"`
	TypeInstance   *string        `json:"type_instance"`
	Values         []*json.Number `json:"values"`
	Message        *string        `json:"message"`
	Meta           map[string]any `json:"meta"`
	Severity       *string        `json:"severity"`
}

type createMetricInfo struct {
	Name   string
	DsType *string
	Val    *json.Number
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

func (cdr *collectDRecord) startTimestamp(metricType string) pcommon.Timestamp {
	if metricType == "cumulative" {
		return pcommon.NewTimestampFromTime(time.Unix(0, int64((*cdr.Time-*cdr.Interval)*float64(time.Second))))
	}
	return pcommon.NewTimestampFromTime(time.Unix(0, 0))
}

func (cdr *collectDRecord) appendToMetrics(logger *zap.Logger, scopeMetrics pmetric.ScopeMetrics, defaultLabels map[string]string) error {
	// Ignore if record is an event instead of data point
	if cdr.isEvent() {
		logger.Debug("ignoring log event", zap.String("message", *cdr.Message))
		return nil
	}

	labels := make(map[string]string, len(defaultLabels))
	for k, v := range defaultLabels {
		labels[k] = v
	}

	for i := range cdr.Dsnames {
		if i < len(cdr.Dstypes) && i < len(cdr.Values) && cdr.Values[i] != nil {
			dsType, dsName, val := cdr.Dstypes[i], cdr.Dsnames[i], cdr.Values[i]
			metricName, usedDsName := cdr.getReasonableMetricName(i, labels)
			createMetric := createMetricInfo{
				Name:   metricName,
				DsType: dsType,
				Val:    val,
			}

			addIfNotNullOrEmpty(labels, "plugin", cdr.Plugin)
			parseAndAddLabels(labels, cdr.PluginInstance, cdr.Host)
			if !usedDsName {
				addIfNotNullOrEmpty(labels, "dsname", dsName)
			}

			metric, err := cdr.newMetric(createMetric, labels)
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
func (cdr *collectDRecord) newMetric(createMetric createMetricInfo, labels map[string]string) (pmetric.Metric, error) {
	attributes := setAttributes(labels)
	metric, err := cdr.setMetric(createMetric, attributes)
	if err != nil {
		return pmetric.Metric{}, fmt.Errorf("error processing metric %s: %w", createMetric.Name, err)
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
func (cdr *collectDRecord) setMetric(createMetric createMetricInfo, atr pcommon.Map) (pmetric.Metric, error) {
	typ := ""
	metric := pmetric.NewMetric()

	if createMetric.DsType != nil {
		typ = *createMetric.DsType
	}

	metric.SetName(createMetric.Name)
	dataPoint := setDataPoint(typ, metric)
	dataPoint.SetTimestamp(cdr.protoTime())
	atr.CopyTo(dataPoint.Attributes())

	if val, err := createMetric.Val.Int64(); err == nil {
		dataPoint.SetIntValue(val)
	} else if val, err := createMetric.Val.Float64(); err == nil {
		dataPoint.SetDoubleValue(val)
	} else {
		return pmetric.Metric{}, fmt.Errorf("value could not be decoded: %w", err)
	}
	return metric, nil
}

// check type to decide metric type and return data point
func setDataPoint(typ string, metric pmetric.Metric) pmetric.NumberDataPoint {
	var dataPoint pmetric.NumberDataPoint
	switch typ {
	case "derive", "counter":
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

	instanceName, extractedAttrs := collectd.LabelsFromName(cdr.TypeInstance)
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
	instanceName, toAddDims := collectd.LabelsFromName(val)

	for k, v := range toAddDims {
		if _, exists := labels[k]; !exists {
			val := v
			addIfNotNullOrEmpty(labels, k, &val)
		}
	}
	addIfNotNullOrEmpty(labels, key, &instanceName)
}
