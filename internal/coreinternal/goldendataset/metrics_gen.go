// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Simple utilities for generating metrics for testing

// MetricsCfg holds parameters for generating dummy metrics for testing. Set values on this struct to generate
// metrics with the corresponding number/type of attributes and pass into MetricsFromCfg to generate metrics.
type MetricsCfg struct {
	// The type of metric to generate
	MetricDescriptorType pmetric.MetricType
	// MetricValueType is the type of the numeric value: int or double.
	MetricValueType pmetric.NumberDataPointValueType
	// If MetricDescriptorType is one of the Sum, this describes if the sum is monotonic or not.
	IsMonotonicSum bool
	// A prefix for every metric name
	MetricNamePrefix string
	// The number of instrumentation library metrics per resource
	NumILMPerResource int
	// The size of the MetricSlice and number of Metrics
	NumMetricsPerILM int
	// The number of labels on the LabelsMap associated with each point
	NumPtLabels int
	// The number of points to generate per Metric
	NumPtsPerMetric int
	// The number of Attributes to insert into each Resource's AttributesMap
	NumResourceAttrs int
	// The number of ResourceMetrics for the single MetricData generated
	NumResourceMetrics int
	// The base value for each point
	PtVal int
	// The start time for each point
	StartTime uint64
	// The duration of the steps between each generated point starting at StartTime
	StepSize uint64
}

// DefaultCfg produces a MetricsCfg with default values. These should be good enough to produce sane
// (but boring) metrics, and can be used as a starting point for making alterations.
func DefaultCfg() MetricsCfg {
	return MetricsCfg{
		MetricDescriptorType: pmetric.MetricTypeGauge,
		MetricValueType:      pmetric.NumberDataPointValueTypeInt,
		MetricNamePrefix:     "",
		NumILMPerResource:    1,
		NumMetricsPerILM:     1,
		NumPtLabels:          1,
		NumPtsPerMetric:      1,
		NumResourceAttrs:     1,
		NumResourceMetrics:   1,
		PtVal:                1,
		StartTime:            940000000000000000,
		StepSize:             42,
	}
}

// MetricsFromCfg produces pmetric.Metrics with the passed-in config.
func MetricsFromCfg(cfg MetricsCfg) pmetric.Metrics {
	mg := newMetricGenerator()
	return mg.genMetricFromCfg(cfg)
}

type metricGenerator struct {
	metricID int
}

func newMetricGenerator() metricGenerator {
	return metricGenerator{}
}

func (g *metricGenerator) genMetricFromCfg(cfg MetricsCfg) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	rms.EnsureCapacity(cfg.NumResourceMetrics)
	for i := 0; i < cfg.NumResourceMetrics; i++ {
		rm := rms.AppendEmpty()
		resource := rm.Resource()
		for j := 0; j < cfg.NumResourceAttrs; j++ {
			resource.Attributes().PutStr(
				fmt.Sprintf("resource-attr-name-%d", j),
				fmt.Sprintf("resource-attr-val-%d", j),
			)
		}
		g.populateIlm(cfg, rm)
	}
	return md
}

func (g *metricGenerator) populateIlm(cfg MetricsCfg, rm pmetric.ResourceMetrics) {
	ilms := rm.ScopeMetrics()
	ilms.EnsureCapacity(cfg.NumILMPerResource)
	for i := 0; i < cfg.NumILMPerResource; i++ {
		ilm := ilms.AppendEmpty()
		g.populateMetrics(cfg, ilm)
	}
}

func (g *metricGenerator) populateMetrics(cfg MetricsCfg, ilm pmetric.ScopeMetrics) {
	metrics := ilm.Metrics()
	metrics.EnsureCapacity(cfg.NumMetricsPerILM)
	for i := 0; i < cfg.NumMetricsPerILM; i++ {
		metric := metrics.AppendEmpty()
		g.populateMetricDesc(cfg, metric)
		switch cfg.MetricDescriptorType {
		case pmetric.MetricTypeGauge:
			populateNumberPoints(cfg, metric.SetEmptyGauge().DataPoints())
		case pmetric.MetricTypeSum:
			sum := metric.SetEmptySum()
			sum.SetIsMonotonic(cfg.IsMonotonicSum)
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			populateNumberPoints(cfg, sum.DataPoints())
		case pmetric.MetricTypeHistogram:
			histo := metric.SetEmptyHistogram()
			histo.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			populateDoubleHistogram(cfg, histo)
		case pmetric.MetricTypeExponentialHistogram:
			histo := metric.SetEmptyExponentialHistogram()
			histo.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			populateExpoHistogram(cfg, histo)
		}
	}
}

func (g *metricGenerator) populateMetricDesc(cfg MetricsCfg, metric pmetric.Metric) {
	metric.SetName(fmt.Sprintf("%smetric_%d", cfg.MetricNamePrefix, g.metricID))
	g.metricID++
	metric.SetDescription("my-md-description")
	metric.SetUnit("my-md-units")
}

func populateNumberPoints(cfg MetricsCfg, pts pmetric.NumberDataPointSlice) {
	pts.EnsureCapacity(cfg.NumPtsPerMetric)
	for i := 0; i < cfg.NumPtsPerMetric; i++ {
		pt := pts.AppendEmpty()
		pt.SetStartTimestamp(pcommon.Timestamp(cfg.StartTime))
		pt.SetTimestamp(getTimestamp(cfg.StartTime, cfg.StepSize, i))
		switch cfg.MetricValueType {
		case pmetric.NumberDataPointValueTypeInt:
			pt.SetIntValue(int64(cfg.PtVal + i))
		case pmetric.NumberDataPointValueTypeDouble:
			pt.SetDoubleValue(float64(cfg.PtVal + i))
		default:
			panic("Should not happen")
		}
		populatePtAttributes(cfg, pt.Attributes())
	}
}

func populateDoubleHistogram(cfg MetricsCfg, dh pmetric.Histogram) {
	pts := dh.DataPoints()
	pts.EnsureCapacity(cfg.NumPtsPerMetric)
	for i := 0; i < cfg.NumPtsPerMetric; i++ {
		pt := pts.AppendEmpty()
		pt.SetStartTimestamp(pcommon.Timestamp(cfg.StartTime))
		ts := getTimestamp(cfg.StartTime, cfg.StepSize, i)
		pt.SetTimestamp(ts)
		populatePtAttributes(cfg, pt.Attributes())
		setDoubleHistogramBounds(pt, 1, 2, 3, 4, 5)
		addDoubleHistogramVal(pt, 1)
		for i := 0; i < cfg.PtVal; i++ {
			addDoubleHistogramVal(pt, 3)
		}
		addDoubleHistogramVal(pt, 5)
	}
}

func setDoubleHistogramBounds(hdp pmetric.HistogramDataPoint, bounds ...float64) {
	counts := make([]uint64, len(bounds))
	hdp.BucketCounts().FromRaw(counts)
	hdp.ExplicitBounds().FromRaw(bounds)
}

func addDoubleHistogramVal(hdp pmetric.HistogramDataPoint, val float64) {
	hdp.SetCount(hdp.Count() + 1)
	hdp.SetSum(hdp.Sum() + val)
	// TODO: HasSum, Min, HasMin, Max, HasMax are not covered in tests.
	buckets := hdp.BucketCounts()
	bounds := hdp.ExplicitBounds()
	for i := 0; i < bounds.Len(); i++ {
		bound := bounds.At(i)
		if val <= bound {
			buckets.SetAt(i, buckets.At(i)+1)
			break
		}
	}
}

func populatePtAttributes(cfg MetricsCfg, lm pcommon.Map) {
	for i := 0; i < cfg.NumPtLabels; i++ {
		k := fmt.Sprintf("pt-label-key-%d", i)
		v := fmt.Sprintf("pt-label-val-%d", i)
		lm.PutStr(k, v)
	}
}

func getTimestamp(startTime uint64, stepSize uint64, i int) pcommon.Timestamp {
	return pcommon.Timestamp(startTime + (stepSize * uint64(i+1)))
}

func populateExpoHistogram(cfg MetricsCfg, dh pmetric.ExponentialHistogram) {
	pts := dh.DataPoints()
	pts.EnsureCapacity(cfg.NumPtsPerMetric)
	for i := 0; i < cfg.NumPtsPerMetric; i++ {
		pt := pts.AppendEmpty()
		pt.SetStartTimestamp(pcommon.Timestamp(cfg.StartTime))
		ts := getTimestamp(cfg.StartTime, cfg.StepSize, i)
		pt.SetTimestamp(ts)
		populatePtAttributes(cfg, pt.Attributes())

		pt.SetSum(100 * float64(cfg.PtVal))
		pt.SetCount(uint64(cfg.PtVal))
		pt.SetScale(int32(cfg.PtVal))
		pt.SetZeroCount(uint64(cfg.PtVal))
		pt.SetMin(float64(cfg.PtVal))
		pt.SetMax(float64(cfg.PtVal))
		pt.Positive().SetOffset(int32(cfg.PtVal))
		pt.Positive().BucketCounts().FromRaw([]uint64{uint64(cfg.PtVal)})
	}
}
