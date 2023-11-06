// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"fmt"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"sort"
	"strings"
	"sync"
	"time"
)

type accumulatedValue struct {
	// value contains a metric with exactly one aggregated datapoint.
	value pmetric.Metric

	// resourceAttrs contain the resource attributes. They are used to output instance and job labels.
	resourceAttrs pcommon.Map

	// updated indicates when metric was last changed.
	updated time.Time
	scope   pcommon.InstrumentationScope
}

// accumulator stores aggragated values of incoming metrics
type accumulator interface {
	// Accumulate stores aggragated metric values
	Accumulate(resourceMetrics pmetric.ResourceMetrics) (processed int)
	// Collect returns a slice with relevant aggregated metrics and their resource attributes.
	// The number or metrics and attributes returned will be the same.
	Collect() (metrics []pmetric.Metric, resourceAttrs []pcommon.Map)
}

// LastValueAccumulator keeps last value for accumulated metrics
type lastValueAccumulator struct {
	logger *zap.Logger

	registeredMetrics sync.Map

	// maxStaleness contains duration for which metric
	// should be served after it was updated
	maxStaleness                          time.Duration
	resetSumOnDeltaStartTimestampMismatch bool
	identifyMode                          IdentifyMode
	telemetry                             *telemetry
}

// NewAccumulator returns LastValueAccumulator
func newAccumulator(logger *zap.Logger, maxStaleness time.Duration, identifyMode IdentifyMode, telemetry *telemetry) accumulator {
	return &lastValueAccumulator{
		logger:       logger,
		maxStaleness: maxStaleness,
		identifyMode: identifyMode,
		telemetry:    telemetry,
	}
}

// Accumulate stores one datapoint per metric
func (a *lastValueAccumulator) Accumulate(rm pmetric.ResourceMetrics) (n int) {
	now := time.Now()
	ilms := rm.ScopeMetrics()
	resourceAttrs := rm.Resource().Attributes()

	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)

		metrics := ilm.Metrics()
		for j := 0; j < metrics.Len(); j++ {
			n += a.addMetric(metrics.At(j), ilm.Scope(), resourceAttrs, now)
		}
	}

	return
}

func (a *lastValueAccumulator) addMetric(metric pmetric.Metric, il pcommon.InstrumentationScope, resourceAttrs pcommon.Map, now time.Time) int {
	a.logger.Debug(fmt.Sprintf("accumulating metric: %s", metric.Name()))

	if metric.Type() != pmetric.MetricTypeSum || metric.Sum().AggregationTemporality() != pmetric.AggregationTemporalityDelta || !metric.Sum().IsMonotonic() {
		return 0
	}

	return a.accumulateSum(metric, il, resourceAttrs, now)
}

func (a *lastValueAccumulator) accumulateSum(metric pmetric.Metric, il pcommon.InstrumentationScope, resourceAttrs pcommon.Map, now time.Time) (n int) {
	doubleSum := metric.Sum()

	dps := doubleSum.DataPoints()

	for i := 0; i < dps.Len(); i++ {
		ip := dps.At(i)

		signature := timeseriesSignature(a.identifyMode, il.Name(), metric, ip.Attributes(), resourceAttrs)

		// equivalent to Prometheus "staleness marker" - indicates we should reset
		if ip.Flags().NoRecordedValue() {
			a.registeredMetrics.Delete(signature)
			a.telemetry.recordStaleTimeseries(signature)
			return 0
		}

		v, ok := a.registeredMetrics.Load(signature)
		if !ok {
			m := copyMetricMetadata(metric)
			m.SetEmptySum().SetIsMonotonic(true)
			m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			dataPoint := m.Sum().DataPoints().AppendEmpty()
			ip.CopyTo(dataPoint)
			dataPoint.SetStartTimestamp(ip.Timestamp())
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
			n++
			a.telemetry.recordNewTimeseries(signature)
			continue
		}
		mv := v.(*accumulatedValue)

		trackingDataPoint := mv.value.Sum().DataPoints().At(0)
		if ip.Timestamp().AsTime().Before(trackingDataPoint.Timestamp().AsTime()) {
			// only keep datapoint with latest timestamp
			a.telemetry.recordDroppedDatapointLate(signature)
			continue
		}

		switch trackingDataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			if ip.ValueType() != pmetric.NumberDataPointValueTypeDouble {
				trackingDataPoint.SetIntValue(trackingDataPoint.IntValue() + ip.IntValue())
			} else {
				// "upgrade" type to double
				trackingDataPoint.SetDoubleValue(trackingDataPoint.DoubleValue() + ip.DoubleValue())
			}
		case pmetric.NumberDataPointValueTypeDouble:
			trackingDataPoint.SetDoubleValue(trackingDataPoint.DoubleValue() + ip.DoubleValue())
		}

		trackingDataPoint.SetTimestamp(ip.Timestamp())

		// TODO check that attributes work as expected, and that the unique set of resource and  datapoint attributes create a unique signature
		a.registeredMetrics.Store(signature, &accumulatedValue{value: mv.value, resourceAttrs: resourceAttrs, scope: il, updated: now})
		n++
	}
	return
}

// Collect returns a slice with relevant aggregated metrics and their resource attributes.
func (a *lastValueAccumulator) Collect() ([]pmetric.Metric, []pcommon.Map) {
	a.logger.Debug("Accumulator collect called")

	var metrics []pmetric.Metric
	var resourceAttrs []pcommon.Map
	expirationTime := time.Now().Add(-a.maxStaleness)

	a.registeredMetrics.Range(func(key, value interface{}) bool {
		v := value.(*accumulatedValue)
		if a.maxStaleness > 0 && expirationTime.After(v.updated) {
			a.logger.Debug(fmt.Sprintf("metric expired: %s", v.value.Name()))
			a.registeredMetrics.Delete(key)
			a.telemetry.recordExpiredTimeseries(key.(string))
			return true
		}
		metrics = append(metrics, v.value)
		resourceAttrs = append(resourceAttrs, v.resourceAttrs)
		return true
	})

	return metrics, resourceAttrs
}

func timeseriesSignature(i IdentifyMode, ilmName string, metric pmetric.Metric, attributes pcommon.Map, resourceAttrs pcommon.Map) string {
	var b strings.Builder

	b.WriteString(replaceSeperator(metric.Type().String()))

	b.WriteString("*")
	b.WriteString(replaceSeperator(ilmName))

	b.WriteString("*")
	b.WriteString(replaceSeperator(metric.Name()))

	if attributes.Len() > 0 {
		b.WriteString("*attrs:")
		attrs := doAttrs(attributes)
		b.WriteString(strings.Join(attrs, "*"))
	}
	if i == ScopeAndResourceAttributes && resourceAttrs.Len() > 0 {
		b.WriteString("*rAttrs:")
		rAttrs := doAttrs(resourceAttrs)
		b.WriteString(strings.Join(rAttrs, "*"))
	}

	return b.String()
}

func doAttrs(attributes pcommon.Map) []string {
	attrs := make([]string, 0, attributes.Len())
	attributes.Range(func(k string, v pcommon.Value) bool {
		attrs = append(attrs, replaceSeperator(k)+"*"+replaceSeperator(v.AsString()))
		return true
	})
	sort.Strings(attrs)
	return attrs
}

func replaceSeperator(s string) string {
	return strings.ReplaceAll(s, "*", "**")
}

func copyMetricMetadata(metric pmetric.Metric) pmetric.Metric {
	m := pmetric.NewMetric()
	m.SetName(metric.Name())
	m.SetDescription(metric.Description())
	m.SetUnit(metric.Unit())

	return m
}
