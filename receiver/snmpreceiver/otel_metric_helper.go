// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
)

// generalResourceKey is the resource key for the no general "no attribute" resource
const generalResourceKey = ""

// getResourceKey returns a unique key based on all of the relevant resource attributes names
// as well as the current row index for a metric data point (a row index corresponds to values
// for the attributes that all belong to a single resource, so we only need this for uniqueness
// and not each value for each attribute name)
//
// For example if we have 3 resource attribute configs: RA1, RA2, RA3 and 1 metric config M1.
// M1 has a single column OID in some table and has references to all 3 resource attribute configs.
// RA1 and RA2 also have an OID in their config which reference different columns in the same table.
// RA3 has an indexed value prefix.
//
// If that table has 3 rows, we are going to expect 3 different resources each with one 1 metric that contains 1 datapoint.
//
// The three resources will have attributes that might look something like this:
// Resource 1
// RA1 => small ("small" is the value at index 1 for RA1's OID column in the table)
// RA2 => cold ("cold" is the value at index 1 for RA2's OID column in the table)
// RA3 => prefix.1 ("prefix.1" is created from RA3's indexed prefix value + the index that corresponds with the related metric's datapoint
// Metric M1 has 1 datapoint with value 10.0 (10.0 is the value at index 1 for M1's column OID in the table)
//
// Resource 2
// RA1 => medium ("medium" is the value at index 2 for RA1's OID column in the table)
// RA2 => temperate ("temperate" is the value at index 2 for RA2's OID column in the table)
// RA3 => prefix.2 ("prefix.2" is created from RA3's indexed prefix value + the index that corresponds with the related metric's datapoint
// Metric M1 has 1 datapoint with value 15.0 (15.0 is the value at index 2 for M1's column OID in the table)
//
// Resource 3
// RA1 => large ("large" is the value at index 3 for RA1's OID column in the table)
// RA2 => hot ("hot" is the value at index 3 for RA2's OID column in the table)
// RA3 => prefix.3 ("prefix.3" is created from RA3's indexed prefix value + the index that corresponds with the related metric's datapoint
// Metric M1 has 1 datapoint with value 5.0 (5.0 is the value at index 3 for M1's column OID in the table)
//
// So we could identify Resource 1 with a string key of "RA1=>small,RA2=>cold,RA3=>prefix.1".
// But we also can uniquely identify Resource 1 with a "shortcut" string key of "RA1,RA2,RA3,1".
// This is because the row index in the table is what is uniquely identifying a single resource within a single collection.
func getResourceKey(
	metricCfgResourceAttributes []string,
	indexString string,
) string {
	sort.Strings(metricCfgResourceAttributes)
	resourceKey := generalResourceKey
	if len(metricCfgResourceAttributes) > 0 {
		resourceKey = strings.Join(metricCfgResourceAttributes, ",") + indexString
	}

	return resourceKey
}

// otelMetricHelper contains many of the functions required to get and create OTEL resources, metrics, and datapoints
type otelMetricHelper struct {
	// This is the metrics that should be returned by scrape
	metrics pmetric.Metrics
	// This is used as an easy reference to grab existing OTEL resources by unique key
	resourcesByKey map[string]*pmetric.ResourceMetrics
	// The info in this map will ultimately already be contained within the resourcesByKey, but it is
	// more easily accessible to pull out a specific existing Metric by resource and metric name using this
	metricsByResource map[string]map[string]*pmetric.Metric
	// This is the ResourceMetricsSlice that will contain all newly created resources and metrics
	resourceMetricsSlice pmetric.ResourceMetricsSlice
	// This is the start timestamp that should be added to all cumulative sum data points
	dataPointStartTime pcommon.Timestamp
	// This is the timestamp that should be added to all created data points
	dataPointTime pcommon.Timestamp
	// This is used so that we can put the proper version on the scope metrics
	settings receiver.CreateSettings
}

// newOtelMetricHelper returns a new otelMetricHelper with an initialized master Metrics
func newOTELMetricHelper(settings receiver.CreateSettings, scraperStartTime pcommon.Timestamp) *otelMetricHelper {
	metrics := pmetric.NewMetrics()
	omh := otelMetricHelper{
		metrics:              metrics,
		resourceMetricsSlice: metrics.ResourceMetrics(),
		resourcesByKey:       map[string]*pmetric.ResourceMetrics{},
		metricsByResource:    map[string]map[string]*pmetric.Metric{},
		dataPointStartTime:   scraperStartTime,
		dataPointTime:        pcommon.NewTimestampFromTime(time.Now()),
		settings:             settings,
	}

	return &omh
}

// getResource returns a resource (if already created) by the resource key
func (h otelMetricHelper) getResource(resourceKey string) *pmetric.ResourceMetrics {
	return h.resourcesByKey[resourceKey]
}

// createResource creates a new resource using the given resource attributes and resource key
func (h *otelMetricHelper) createResource(resourceKey string, resourceAttributes map[string]string) *pmetric.ResourceMetrics {
	resourceMetrics := h.resourceMetricsSlice.AppendEmpty()
	for key, value := range resourceAttributes {
		resourceMetrics.Resource().Attributes().PutStr(key, value)
	}
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("otelcol/snmpreceiver")
	scopeMetrics.Scope().SetVersion(h.settings.BuildInfo.Version)
	h.resourcesByKey[resourceKey] = &resourceMetrics
	h.metricsByResource[resourceKey] = map[string]*pmetric.Metric{}

	return &resourceMetrics
}

// getMetric returns a metric (if already created) by resource key and metric name
func (h otelMetricHelper) getMetric(resourceKey string, metricName string) *pmetric.Metric {
	if h.metricsByResource[resourceKey] == nil {
		h.metricsByResource[resourceKey] = map[string]*pmetric.Metric{}
	}

	return h.metricsByResource[resourceKey][metricName]
}

// createResource creates a new metric using on the resource key'd resource using the given metric config data
func (h *otelMetricHelper) createMetric(resourceKey string, metricName string, metricCfg *MetricConfig) (*pmetric.Metric, error) {
	resource := h.getResource(resourceKey)
	if resource == nil {
		return nil, fmt.Errorf("cannot create metric '%s' as no resource exists for it to be attached", metricName)
	}
	metricSlice := resource.ScopeMetrics().At(0).Metrics()
	newMetric := metricSlice.AppendEmpty()
	newMetric.SetName(metricName)
	newMetric.SetDescription(metricCfg.Description)
	newMetric.SetUnit(metricCfg.Unit)

	if metricCfg.Sum != nil {
		newMetric.SetEmptySum()
		newMetric.Sum().SetIsMonotonic(metricCfg.Sum.Monotonic)

		switch metricCfg.Sum.Aggregation {
		case "cumulative":
			newMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		case "delta":
			newMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
	} else {
		newMetric.SetEmptyGauge()
	}
	h.metricsByResource[resourceKey][metricName] = &newMetric

	return &newMetric, nil
}

// addMetricDataPoint creates a datapoint on the metric (metricName) attached to a resource (resourceKey) and populates it
// based on the given data
func (h *otelMetricHelper) addMetricDataPoint(resourceKey string, metricName string, metricCfg *MetricConfig, data SNMPData, attributes map[string]string) (*pmetric.NumberDataPoint, error) {
	metric := h.getMetric(resourceKey, metricName)
	if metric == nil {
		return nil, fmt.Errorf("cannot retrieve datapoints from metric '%s' as it does not currently exist", metricName)
	}

	var dp pmetric.NumberDataPoint
	var valueType string
	if metricCfg.Gauge != nil {
		dp = metric.Gauge().DataPoints().AppendEmpty()
		valueType = metricCfg.Gauge.ValueType
	} else {
		dp = metric.Sum().DataPoints().AppendEmpty()
		dp.SetStartTimestamp(h.dataPointStartTime)
		valueType = metricCfg.Sum.ValueType
	}

	// Creates a data point based on the SNMP data
	dp.SetTimestamp(h.dataPointTime)

	// Not explicitly checking these casts as this should be made safe in the client
	switch data.valueType {
	case floatVal:
		rawValue := data.value.(float64)
		if valueType == "double" {
			dp.SetDoubleValue(rawValue)
		} else {
			dp.SetIntValue(int64(rawValue))
		}
	case integerVal:
		rawValue := data.value.(int64)
		if valueType == "int" {
			dp.SetIntValue(rawValue)
		} else {
			dp.SetDoubleValue(float64(rawValue))
		}
	case stringVal:
		return nil, fmt.Errorf("cannot create data point for metric %q from string value", metricName)
	case notSupportedVal:
		return nil, fmt.Errorf("cannot create data point for metric %q from unsupported value type", metricName)
	}

	// Add attributes to dp
	for key, value := range attributes {
		dp.Attributes().PutStr(key, value)
	}

	return &dp, nil
}
