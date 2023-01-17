// Copyright The OpenTelemetry Authors
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

package comparetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/comparetest"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// MetricsCompareOption can be used to mutate expected and/or actual metrics before comparing.
type MetricsCompareOption interface {
	applyOnMetrics(expected, actual pmetric.Metrics)
}

// LogsCompareOption can be used to mutate expected and/or actual logs before comparing.
type LogsCompareOption interface {
	applyOnLogs(expected, actual plog.Logs)
}

// TracesCompareOption can be used to mutate expected and/or actual traces before comparing.
type TracesCompareOption interface {
	applyOnTraces(expected, actual ptrace.Traces)
}

type CompareOption interface {
	MetricsCompareOption
	LogsCompareOption
	TracesCompareOption
}

// IgnoreMetricValues is a MetricsCompareOption that clears all metric values.
func IgnoreMetricValues(metricNames ...string) MetricsCompareOption {
	return ignoreMetricValues{
		metricNames: metricNames,
	}
}

type ignoreMetricValues struct {
	metricNames []string
}

func (opt ignoreMetricValues) applyOnMetrics(expected, actual pmetric.Metrics) {
	maskMetricValues(expected, opt.metricNames...)
	maskMetricValues(actual, opt.metricNames...)
}

func maskMetricValues(metrics pmetric.Metrics, metricNames ...string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			maskMetricSliceValues(ilms.At(j).Metrics(), metricNames...)
		}
	}
}

// maskMetricSliceValues sets all data point values to zero.
func maskMetricSliceValues(metrics pmetric.MetricSlice, metricNames ...string) {
	metricNameSet := make(map[string]bool, len(metricNames))
	for _, metricName := range metricNames {
		metricNameSet[metricName] = true
	}
	for i := 0; i < metrics.Len(); i++ {
		if len(metricNames) == 0 || metricNameSet[metrics.At(i).Name()] {
			maskDataPointSliceValues(getDataPointSlice(metrics.At(i)))
		}
	}
}

// maskDataPointSliceValues sets all data point values to zero.
func maskDataPointSliceValues(dataPoints pmetric.NumberDataPointSlice) {
	for i := 0; i < dataPoints.Len(); i++ {
		dataPoint := dataPoints.At(i)
		dataPoint.SetIntValue(0)
		dataPoint.SetDoubleValue(0)
	}
}

// IgnoreMetricAttributeValue is a MetricsCompareOption that clears value of the metric attribute.
func IgnoreMetricAttributeValue(attributeName string, metricNames ...string) MetricsCompareOption {
	return ignoreMetricAttributeValue{
		attributeName: attributeName,
		metricNames:   metricNames,
	}
}

type ignoreMetricAttributeValue struct {
	attributeName string
	metricNames   []string
}

func (opt ignoreMetricAttributeValue) applyOnMetrics(expected, actual pmetric.Metrics) {
	maskMetricAttributeValue(expected, opt)
	maskMetricAttributeValue(actual, opt)
}

func maskMetricAttributeValue(metrics pmetric.Metrics, opt ignoreMetricAttributeValue) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			maskMetricSliceAttributeValues(ilms.At(j).Metrics(), opt.attributeName, opt.metricNames...)
		}
	}
}

// maskMetricSliceAttributeValues sets the value of the specified attribute to
// the zero value associated with the attribute data type.
// If metric names are specified, only the data points within those metrics will be masked.
// Otherwise, all data points with the attribute will be masked.
func maskMetricSliceAttributeValues(metrics pmetric.MetricSlice, attributeName string, metricNames ...string) {
	metricNameSet := make(map[string]bool, len(metricNames))
	for _, metricName := range metricNames {
		metricNameSet[metricName] = true
	}

	for i := 0; i < metrics.Len(); i++ {
		if len(metricNames) == 0 || metricNameSet[metrics.At(i).Name()] {
			dps := getDataPointSlice(metrics.At(i))
			maskDataPointSliceAttributeValues(dps, attributeName)

			// If attribute values are ignored, some data points may become
			// indistinguishable from each other, but sorting by value allows
			// for a reasonably thorough comparison and a deterministic outcome.
			dps.Sort(func(a, b pmetric.NumberDataPoint) bool {
				if a.IntValue() < b.IntValue() {
					return true
				}
				if a.DoubleValue() < b.DoubleValue() {
					return true
				}
				return false
			})
		}
	}
}

// maskDataPointSliceAttributeValues sets the value of the specified attribute to
// the zero value associated with the attribute data type.
func maskDataPointSliceAttributeValues(dataPoints pmetric.NumberDataPointSlice, attributeName string) {
	for i := 0; i < dataPoints.Len(); i++ {
		attributes := dataPoints.At(i).Attributes()
		attribute, ok := attributes.Get(attributeName)
		if ok {
			switch attribute.Type() {
			case pcommon.ValueTypeStr:
				attribute.SetStr("")
			default:
				panic(fmt.Sprintf("data type not supported: %s", attribute.Type()))
			}
		}
	}
}

// IgnoreResourceAttributeValue is a CompareOption that removes a resource attribute
// from all resources.
func IgnoreResourceAttributeValue(attributeName string) CompareOption {
	return ignoreResourceAttributeValue{
		attributeName: attributeName,
	}
}

type ignoreResourceAttributeValue struct {
	attributeName string
}

func (opt ignoreResourceAttributeValue) applyOnMetrics(expected, actual pmetric.Metrics) {
	opt.maskMetricsResourceAttributeValue(expected)
	opt.maskMetricsResourceAttributeValue(actual)
}

func (opt ignoreResourceAttributeValue) maskMetricsResourceAttributeValue(metrics pmetric.Metrics) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		opt.maskResourceAttributeValue(rms.At(i).Resource())
	}
}

func (opt ignoreResourceAttributeValue) applyOnLogs(expected, actual plog.Logs) {
	opt.maskLogsResourceAttributeValue(expected)
	opt.maskLogsResourceAttributeValue(actual)
}

func (opt ignoreResourceAttributeValue) maskLogsResourceAttributeValue(metrics plog.Logs) {
	rls := metrics.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		opt.maskResourceAttributeValue(rls.At(i).Resource())
	}
}

func (opt ignoreResourceAttributeValue) applyOnTraces(expected, actual ptrace.Traces) {
	opt.maskTracesResourceAttributeValue(expected)
	opt.maskTracesResourceAttributeValue(actual)
}

func (opt ignoreResourceAttributeValue) maskTracesResourceAttributeValue(traces ptrace.Traces) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		opt.maskResourceAttributeValue(rss.At(i).Resource())
	}
}

func (opt ignoreResourceAttributeValue) maskResourceAttributeValue(res pcommon.Resource) {
	if _, ok := res.Attributes().Get(opt.attributeName); ok {
		res.Attributes().Remove(opt.attributeName)
	}
}

// IgnoreSubsequentDataPoints is a MetricsCompareOption that ignores data points after the first.
func IgnoreSubsequentDataPoints(metricNames ...string) MetricsCompareOption {
	return ignoreSubsequentDataPoints{
		metricNames: metricNames,
	}
}

type ignoreSubsequentDataPoints struct {
	metricNames []string
}

func (opt ignoreSubsequentDataPoints) applyOnMetrics(expected, actual pmetric.Metrics) {
	maskSubsequentDataPoints(expected, opt.metricNames...)
	maskSubsequentDataPoints(actual, opt.metricNames...)
}

func maskSubsequentDataPoints(metrics pmetric.Metrics, metricNames ...string) {
	metricNameSet := make(map[string]bool, len(metricNames))
	for _, metricName := range metricNames {
		metricNameSet[metricName] = true
	}

	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				if len(metricNames) == 0 || metricNameSet[ms.At(k).Name()] {
					dps := getDataPointSlice(ms.At(k))
					n := 0
					dps.RemoveIf(func(pmetric.NumberDataPoint) bool {
						n++
						return n > 1
					})
				}
			}
		}
	}
}

func IgnoreObservedTimestamp() LogsCompareOption {
	return ignoreObservedTimestamp{}
}

type ignoreObservedTimestamp struct{}

func (opt ignoreObservedTimestamp) applyOnLogs(expected, actual plog.Logs) {
	now := pcommon.NewTimestampFromTime(time.Now())
	maskObservedTimestamp(expected, now)
	maskObservedTimestamp(actual, now)
}

func maskObservedTimestamp(logs plog.Logs, ts pcommon.Timestamp) {
	rls := logs.ResourceLogs()
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				lrs.At(k).SetObservedTimestamp(ts)
			}
		}
	}
}

// IgnoreResourceOrder is a CompareOption that ignores the order of resource traces/metrics/logs.
func IgnoreResourceOrder() CompareOption {
	return ignoreResourceOrder{}
}

type ignoreResourceOrder struct{}

func (opt ignoreResourceOrder) applyOnTraces(expected, actual ptrace.Traces) {
	expected.ResourceSpans().Sort(sortResourceSpans)
	actual.ResourceSpans().Sort(sortResourceSpans)
}

func (opt ignoreResourceOrder) applyOnMetrics(expected, actual pmetric.Metrics) {
	expected.ResourceMetrics().Sort(sortResourceMetrics)
	actual.ResourceMetrics().Sort(sortResourceMetrics)
}

func (opt ignoreResourceOrder) applyOnLogs(expected, actual plog.Logs) {
	expected.ResourceLogs().Sort(sortResourceLogs)
	actual.ResourceLogs().Sort(sortResourceLogs)
}
