// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/prometheus/otlptranslator"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	prom "github.com/prometheus/prometheus/storage/remote/otlptranslator/prometheusremotewrite"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

// FromMetricsV2 converts pmetric.Metrics to Prometheus remote write format 2.0.
func FromMetricsV2(md pmetric.Metrics, settings Settings) (map[string]*writev2.TimeSeries, writev2.SymbolsTable, error) {
	c := newPrometheusConverterV2(settings)
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
	// conflicts is a map of time series signatures(an unique identifier for TS labels) to a list of TSs with the same signature.
	// this is used to handle conflicts that occur when multiple TSs have the same labels or when different labels generate the same signature.
	conflicts map[uint64][]*writev2.TimeSeries
	// conflictCount is used to track the number of conflicts that were encountered.
	conflictCount int
	symbolTable   writev2.SymbolsTable

	metricNamer otlptranslator.MetricNamer
	labelNamer  otlptranslator.LabelNamer
	unitNamer   otlptranslator.UnitNamer

	// Track scopes for otel_scope_info generation
	scopes map[uint64]scopeInfo
}

type metadata struct {
	Type writev2.Metadata_MetricType
	Help string
	Unit string
}

func newPrometheusConverterV2(settings Settings) *prometheusConverterV2 {
	return &prometheusConverterV2{
		unique:      map[uint64]*writev2.TimeSeries{},
		conflicts:   map[uint64][]*writev2.TimeSeries{},
		symbolTable: writev2.NewSymbolTable(),
		metricNamer: otlptranslator.MetricNamer{WithMetricSuffixes: settings.AddMetricSuffixes, Namespace: settings.Namespace},
		labelNamer:  otlptranslator.LabelNamer{UnderscoreLabelSanitization: !prometheus.DropSanitizationGate.IsEnabled()},
		unitNamer:   otlptranslator.UnitNamer{},
		scopes:      map[uint64]scopeInfo{},
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

		// Track scope signatures for this resource to generate scope info later
		resourceScopeSignatures := make(map[uint64]bool)

		for j := 0; j < scopeMetricsSlice.Len(); j++ {
			scopeMetrics := scopeMetricsSlice.At(j)
			metricSlice := scopeMetrics.Metrics()

			scope := scopeMetrics.Scope()
			scopeName := scope.Name()
			scopeVersion := scope.Version()
			scopeAttrs := scope.Attributes()

			// Generate scope signature for deduplication
			scopeSignature := getScopeSignature(scopeName, scopeVersion, scopeAttrs, resource)

			// Track this scope signature for the current resource
			resourceScopeSignatures[scopeSignature] = true

			// Store scope information for later processing (otel_scope_info generation)
			if _, exists := c.scopes[scopeSignature]; !exists {
				c.scopes[scopeSignature] = scopeInfo{
					name:       scopeName,
					version:    scopeVersion,
					attributes: scopeAttrs,
					resource:   resource,
					timestamp:  0, // Will be updated with most recent timestamp
				}
			}

			for k := 0; k < metricSlice.Len(); k++ {
				metric := metricSlice.At(k)
				metricTimestamp := mostRecentTimestampInMetric(metric)
				mostRecentTimestamp = max(mostRecentTimestamp, metricTimestamp)

				// Update the scope's most recent timestamp
				if storedScopeInfo, exists := c.scopes[scopeSignature]; exists {
					storedScopeInfo.timestamp = max(storedScopeInfo.timestamp, metricTimestamp)
					c.scopes[scopeSignature] = storedScopeInfo
				}

				if !isValidAggregationTemporality(metric) {
					errs = multierr.Append(errs, fmt.Errorf("invalid temporality and type combination for metric %q", metric.Name()))
					continue
				}

				promName, err := c.metricNamer.Build(prom.TranslatorMetricFromOtelMetric(metric))
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				m := metadata{
					Type: otelMetricTypeToPromMetricTypeV2(metric),
					Help: metric.Description(),
					Unit: c.unitNamer.Build(metric.Unit()),
				}

				// handle individual metrics based on type
				//exhaustive:enforce
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					dataPoints := metric.Gauge().DataPoints()
					if dataPoints.Len() == 0 {
						break
					}
					errs = multierr.Append(errs, c.addGaugeNumberDataPoints(dataPoints, resource, settings, promName, m, scopeName, scopeVersion))
				case pmetric.MetricTypeSum:
					dataPoints := metric.Sum().DataPoints()
					if dataPoints.Len() == 0 {
						break
					}
					if !metric.Sum().IsMonotonic() {
						errs = multierr.Append(errs, c.addGaugeNumberDataPoints(dataPoints, resource, settings, promName, m, scopeName, scopeVersion))
					} else {
						errs = multierr.Append(errs, c.addSumNumberDataPoints(dataPoints, resource, metric, settings, promName, m, scopeName, scopeVersion))
					}
				case pmetric.MetricTypeHistogram:
					dataPoints := metric.Histogram().DataPoints()
					if dataPoints.Len() == 0 {
						break
					}
					errs = multierr.Append(errs, c.addHistogramDataPoints(dataPoints, resource, settings, promName, m, scopeName, scopeVersion))
				case pmetric.MetricTypeExponentialHistogram:
					dataPoints := metric.ExponentialHistogram().DataPoints()
					if dataPoints.Len() == 0 {
						break
					}
					errs = multierr.Append(errs, c.addExponentialHistogramDataPoints(
						dataPoints, resource, settings, promName, m, scopeName, scopeVersion))
				case pmetric.MetricTypeSummary:
					dataPoints := metric.Summary().DataPoints()
					if dataPoints.Len() == 0 {
						break
					}
					errs = multierr.Append(errs, c.addSummaryDataPoints(dataPoints, resource, settings, promName, m, scopeName, scopeVersion))
				default:
					errs = multierr.Append(errs, errors.New("unsupported metric type"))
				}
			}
		}

		// Generate otel_scope_info metrics for each unique scope in this resource
		for signature := range resourceScopeSignatures {
			if storedScopeInfo, exists := c.scopes[signature]; exists {
				err := c.addScopeInfoV2(storedScopeInfo.name, storedScopeInfo.version, storedScopeInfo.attributes,
					resource, settings, storedScopeInfo.timestamp)
				if err != nil {
					errs = multierr.Append(errs, fmt.Errorf("failed to add scope info for scope %q: %w", storedScopeInfo.name, err))
				}
			}
		}

		errs = multierr.Append(errs, c.addResourceTargetInfoV2(resource, settings, mostRecentTimestamp))
	}

	return
}

// timeSeries returns a slice of the writev2.TimeSeries that were converted from OTel format.
func (c *prometheusConverterV2) timeSeries() []writev2.TimeSeries {
	allTS := make([]writev2.TimeSeries, 0, len(c.unique)+c.conflictCount)
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

func (c *prometheusConverterV2) addSample(sample *writev2.Sample, lbls []prompb.Label, metadata metadata) {
	ts := c.getOrCreateTimeSeries(lbls, metadata)
	ts.Samples = append(ts.Samples, *sample)
}

// isSameMetricV2 checks if two time series are the same metric
func isSameMetricV2(ts1, ts2 *writev2.TimeSeries) bool {
	if len(ts1.LabelsRefs) != len(ts2.LabelsRefs) {
		return false
	}
	for i := 0; i < len(ts1.LabelsRefs); i++ {
		if ts1.LabelsRefs[i] != ts2.LabelsRefs[i] {
			return false
		}
	}
	return true
}

// getOrCreateTimeSeries returns the time series corresponding to the label set
func (c *prometheusConverterV2) getOrCreateTimeSeries(lbls []prompb.Label, metadata metadata) *writev2.TimeSeries {
	signature := timeSeriesSignature(lbls)
	ts := c.unique[signature]
	buf := make([]uint32, 0, len(lbls)*2)

	for _, l := range lbls {
		off := c.symbolTable.Symbolize(l.Name)
		buf = append(buf, off)
		off = c.symbolTable.Symbolize(l.Value)
		buf = append(buf, off)
	}

	ts2 := &writev2.TimeSeries{
		LabelsRefs: buf,
		Metadata: writev2.Metadata{
			Type:    metadata.Type,
			HelpRef: c.symbolTable.Symbolize(metadata.Help),
			UnitRef: c.symbolTable.Symbolize(metadata.Unit),
		},
	}

	if ts != nil {
		if isSameMetricV2(ts, ts2) {
			// We already have this metric
			return ts
		}

		// Look for a matching conflict
		for _, cTS := range c.conflicts[signature] {
			if isSameMetricV2(cTS, ts2) {
				// We already have this metric
				return cTS
			}
		}

		// New conflict
		c.conflicts[signature] = append(c.conflicts[signature], ts2)
		c.conflictCount++
		return ts2
	}

	// This metric is new
	c.unique[signature] = ts2
	return ts2
}
