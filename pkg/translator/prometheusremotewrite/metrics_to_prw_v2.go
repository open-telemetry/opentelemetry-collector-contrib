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
	// TODO handle conflicts
	unique      map[uint64]*writev2.TimeSeries
	symbolTable writev2.SymbolsTable
}

type metadata struct {
	Type writev2.Metadata_MetricType
	Help string
	Unit string
}

func newPrometheusConverterV2() *prometheusConverterV2 {
	return &prometheusConverterV2{
		unique:      map[uint64]*writev2.TimeSeries{},
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
				m := metadata{
					Type: otelMetricTypeToPromMetricTypeV2(metric),
					Help: metric.Description(),
					Unit: prometheustranslator.BuildCompliantPrometheusUnit(metric.Unit()),
				}

				// handle individual metrics based on type
				//exhaustive:enforce
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					dataPoints := metric.Gauge().DataPoints()
					if dataPoints.Len() == 0 {
						break
					}
					c.addGaugeNumberDataPoints(dataPoints, resource, settings, promName, m)
				case pmetric.MetricTypeSum:
					dataPoints := metric.Sum().DataPoints()
					if dataPoints.Len() == 0 {
						break
					}
					if !metric.Sum().IsMonotonic() {
						c.addGaugeNumberDataPoints(dataPoints, resource, settings, promName, m)
					} else {
						c.addSumNumberDataPoints(dataPoints, resource, metric, settings, promName, m)
					}
				case pmetric.MetricTypeHistogram:
					// TODO implement
				case pmetric.MetricTypeExponentialHistogram:
					// TODO implement
				case pmetric.MetricTypeSummary:
					dataPoints := metric.Summary().DataPoints()
					if dataPoints.Len() == 0 {
						break
					}
					c.addSummaryDataPoints(dataPoints, resource, settings, promName, m)
				default:
					errs = multierr.Append(errs, errors.New("unsupported metric type"))
				}
			}
		}
		c.addResourceTargetInfoV2(resource, settings, mostRecentTimestamp)
	}

	return
}

// timeSeries returns a slice of the writev2.TimeSeries that were converted from OTel format.
func (c *prometheusConverterV2) timeSeries() []writev2.TimeSeries {
	allTS := make([]writev2.TimeSeries, 0, len(c.unique))
	for _, ts := range c.unique {
		allTS = append(allTS, *ts)
	}
	return allTS
}

func (c *prometheusConverterV2) addSample(sample *writev2.Sample, lbls []prompb.Label, metadata metadata) {
	// TODO consider how to accommodate metadata in the symbol table when allocating the buffer, given not all metrics might have metadata.
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
	ts := writev2.TimeSeries{
		LabelsRefs: buf,
		Samples:    []writev2.Sample{*sample},
		Metadata: writev2.Metadata{
			Type:    metadata.Type,
			HelpRef: c.symbolTable.Symbolize(metadata.Help),
			UnitRef: c.symbolTable.Symbolize(metadata.Unit),
		},
	}
	c.unique[timeSeriesSignature(lbls)] = &ts
}
