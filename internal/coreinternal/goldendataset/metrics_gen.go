// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	MetricDescriptorType pmetric.MetricDataType
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
		MetricDescriptorType: pmetric.MetricDataTypeGauge,
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
			resource.Attributes().Insert(
				fmt.Sprintf("resource-attr-name-%d", j),
				pcommon.NewValueString(fmt.Sprintf("resource-attr-val-%d", j)),
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
		case pmetric.MetricDataTypeGauge:
			metric.SetDataType(pmetric.MetricDataTypeGauge)
			populateNumberPoints(cfg, metric.Gauge().DataPoints())
		case pmetric.MetricDataTypeSum:
			metric.SetDataType(pmetric.MetricDataTypeSum)
			sum := metric.Sum()
			sum.SetIsMonotonic(cfg.IsMonotonicSum)
			sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
			populateNumberPoints(cfg, sum.DataPoints())
		case pmetric.MetricDataTypeHistogram:
			metric.SetDataType(pmetric.MetricDataTypeHistogram)
			histo := metric.Histogram()
			histo.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
			populateDoubleHistogram(cfg, histo)
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
			pt.SetIntVal(int64(cfg.PtVal + i))
		case pmetric.NumberDataPointValueTypeDouble:
			pt.SetDoubleVal(float64(cfg.PtVal + i))
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
	hdp.SetBucketCounts(pcommon.NewImmutableUInt64Slice(counts))
	hdp.SetExplicitBounds(pcommon.NewImmutableFloat64Slice(bounds))
}

func addDoubleHistogramVal(hdp pmetric.HistogramDataPoint, val float64) {
	hdp.SetCount(hdp.Count() + 1)
	hdp.SetSum(hdp.Sum() + val)
	buckets := hdp.BucketCounts().AsRaw()
	bounds := hdp.ExplicitBounds()
	for i := 0; i < bounds.Len(); i++ {
		bound := bounds.At(i)
		if val <= bound {
			buckets[i]++
			break
		}
	}
	hdp.SetBucketCounts(pcommon.NewImmutableUInt64Slice(buckets))
}

func populatePtAttributes(cfg MetricsCfg, lm pcommon.Map) {
	for i := 0; i < cfg.NumPtLabels; i++ {
		k := fmt.Sprintf("pt-label-key-%d", i)
		v := fmt.Sprintf("pt-label-val-%d", i)
		lm.InsertString(k, v)
	}
}

func getTimestamp(startTime uint64, stepSize uint64, i int) pcommon.Timestamp {
	return pcommon.Timestamp(startTime + (stepSize * uint64(i+1)))
}
