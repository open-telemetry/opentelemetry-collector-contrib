// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

// FromMetricsV2 converts pmetric.Metrics to Prometheus remote write format 2.0.
func FromMetricsV2(md pmetric.Metrics, settings Settings) (map[string]*writev2.TimeSeries, writev2.SymbolsTable, error) {
	c := newPrometheusConverterV2()
	errs := c.fromMetrics(md, settings)
	tss := c.timeSeries()
	out := make(map[string]*writev2.TimeSeries, len(tss))
	for i := range tss {
		out[strconv.Itoa(i)] = &tss[i]
	}

	return out, c.symbolTable, errs
}

// prometheusConverterV2 converts from OTLP to Prometheus write 2.0 format.
type prometheusConverterV2 struct {
	unique map[uint64]*writev2.TimeSeries
	// conflicts is a map of time series signatures(an unique identifier for TS labels) to a list of TSs with the same signature
	// this is used to handle conflicts, that occur when multiple TSs have the same labels
	conflicts   map[uint64][]*writev2.TimeSeries
	symbolTable writev2.SymbolsTable
}

func newPrometheusConverterV2() *prometheusConverterV2 {
	return &prometheusConverterV2{
		unique:      map[uint64]*writev2.TimeSeries{},
		conflicts:   map[uint64][]*writev2.TimeSeries{},
		symbolTable: writev2.NewSymbolTable(),
	}
}

// fromMetrics converts pmetric.Metrics to Prometheus remote write format.
func (c *prometheusConverterV2) fromMetrics(md pmetric.Metrics, settings Settings) (errs error) {
	resourceMetricsSlice := md.ResourceMetrics()
	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		resourceMetrics := resourceMetricsSlice.At(i)
		resource := resourceMetrics.Resource()
		scopeMetricsSlice := resourceMetrics.ScopeMetrics()
		// keep track of the most recent timestamp in the ResourceMetrics for
		// use with the "target" info metric
		var mostRecentTimestamp pcommon.Timestamp
		for j := 0; j < scopeMetricsSlice.Len(); j++ {
			metricSlice := scopeMetricsSlice.At(j).Metrics()

			// TODO: decide if instrumentation library information should be exported as labels
			for k := 0; k < metricSlice.Len(); k++ {
				metric := metricSlice.At(k)
				mostRecentTimestamp = max(mostRecentTimestamp, mostRecentTimestampInMetric(metric))

				if !isValidAggregationTemporality(metric) {
					errs = multierr.Append(errs, fmt.Errorf("invalid temporality and type combination for metric %q", metric.Name()))
					continue
				}

				promName := prometheustranslator.BuildCompliantName(metric, settings.Namespace, settings.AddMetricSuffixes)

				// handle individual metrics based on type
				//exhaustive:enforce
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					dataPoints := metric.Gauge().DataPoints()
					if dataPoints.Len() == 0 {
						break
					}
					c.addGaugeNumberDataPoints(dataPoints, resource, settings, promName)
				case pmetric.MetricTypeSum:
					dataPoints := metric.Sum().DataPoints()
					if dataPoints.Len() == 0 {
						break
					}
					if !metric.Sum().IsMonotonic() {
						c.addGaugeNumberDataPoints(dataPoints, resource, settings, promName)
					} else {
						c.addSumNumberDataPoints(dataPoints, resource, metric, settings, promName)
					}
				case pmetric.MetricTypeHistogram:
					// TODO implement
				case pmetric.MetricTypeExponentialHistogram:
					// TODO implement
				case pmetric.MetricTypeSummary:
					// TODO implement
				default:
					errs = multierr.Append(errs, errors.New("unsupported metric type"))
				}
			}
		}
		// TODO implement
		// addResourceTargetInfov2(resource, settings, mostRecentTimestamp, c)
	}

	return
}

// timeSeries returns a slice of the writev2.TimeSeries that were converted from OTel format.
func (c *prometheusConverterV2) timeSeries() []writev2.TimeSeries {
	conflicts := 0
	for _, ts := range c.conflicts {
		conflicts += len(ts)
	}
	allTS := make([]writev2.TimeSeries, 0, len(c.unique)+conflicts)
	for _, ts := range c.unique {
		allTS = append(allTS, *ts)
	}
	for _, cTS := range c.conflicts {
		for _, ts := range cTS {
			allTS = append(allTS, *ts)
		}
	}
	return allTS
}

func (c *prometheusConverterV2) addSample(sample *writev2.Sample, lbls []prompb.Label) {
	buf := make([]uint32, 0, len(lbls)*2)

	// TODO: Read the PRW spec to see if labels need to be sorted. If it is, then we need to sort in export code. If not, we can sort in the test. (@dashpole have more context on this)
	sort.Slice(lbls, func(i, j int) bool {
		return lbls[i].Name < lbls[j].Name
	})

	var off uint32
	for _, l := range lbls {
		off = c.symbolTable.Symbolize(l.Name)
		buf = append(buf, off)
		off = c.symbolTable.Symbolize(l.Value)
		buf = append(buf, off)
	}

	sig := timeSeriesSignature(lbls)
	ts := &writev2.TimeSeries{
		LabelsRefs: buf,
		Samples:    []writev2.Sample{*sample},
	}

	// check if the time series is already in the unique map
	if existingTS, ok := c.unique[sig]; ok {
		// if the time series is already in the unique map, check if it is the same metric
		if !isSameMetricV2(existingTS, ts) {
			// if the time series is not the same metric, add it to the conflicts map
			c.conflicts[sig] = append(c.conflicts[sig], ts)
		} else {
			// if the time series is the same metric, add the sample to the existing time series
			existingTS.Samples = append(existingTS.Samples, *sample)
		}
	} else {
		// if the time series is not in the unique map, add it to the unique map
		c.unique[sig] = ts
	}
}

// isSameMetricV2 checks if two time series are the same metric
func isSameMetricV2(ts1, ts2 *writev2.TimeSeries) bool {
	if len(ts1.LabelsRefs) != len(ts2.LabelsRefs) {
		return false
	}
	// As the labels are sorted as name, value, name, value, ... we can compare the labels by index jumping 2 steps at a time
	for i := 0; i < len(ts1.LabelsRefs); i += 2 {
		if ts1.LabelsRefs[i] != ts2.LabelsRefs[i] || ts1.LabelsRefs[i+1] != ts2.LabelsRefs[i+1] {
			return false
		}
	}
	return true
}
