package metrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Metric struct {
	res   pcommon.Resource
	scope pcommon.InstrumentationScope
	pmetric.Metric
}

func (m *Metric) Meta() Meta {
	meta := Meta{
		resource: m.res,
		scope:    m.scope,
	}
	m.CopyTo(meta.metric)
	metric := &meta.metric

	// drop samples, gather intrinsics
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		sum := metric.Sum()
		meta.monotonic = sum.IsMonotonic()
		meta.temporality = sum.AggregationTemporality()
		metric.SetEmptySum()
	case pmetric.MetricTypeGauge:
		metric.SetEmptyGauge()
	case pmetric.MetricTypeHistogram:
		meta.temporality = metric.Histogram().AggregationTemporality()
		meta.monotonic = true
		metric.SetEmptyHistogram()
	case pmetric.MetricTypeExponentialHistogram:
		meta.temporality = metric.ExponentialHistogram().AggregationTemporality()
		meta.monotonic = true
		metric.SetEmptyExponentialHistogram()
	}

	return meta
}

// type Metric struct {
// 	Meta
// 	Data
// }

// type Data struct {
// 	Type pmetric.MetricType

// 	Sum   pmetric.Sum
// 	Gauge pmetric.Gauge
// 	Hist  pmetric.Histogram
// 	Exp   pmetric.ExponentialHistogram
// }

func From(res pcommon.Resource, scope pcommon.InstrumentationScope, metric pmetric.Metric) Metric {
	return Metric{res: res, scope: scope, Metric: metric}
	// var data Data
	// meta := Meta{
	// 	resource: res,
	// 	scope:    scope,
	// }

	// metric.CopyTo(meta.metric)
	// metric = meta.metric

	// // split meta and data
	// switch metric.Type() {
	// case pmetric.MetricTypeSum:
	// 	data.Sum = metric.Sum()
	// 	meta.monotonic = data.Sum.IsMonotonic()
	// 	meta.temporality = data.Sum.AggregationTemporality()
	// 	metric.SetEmptySum()
	// case pmetric.MetricTypeGauge:
	// 	data.Gauge = metric.Gauge()
	// 	metric.SetEmptyGauge()
	// case pmetric.MetricTypeHistogram:
	// 	data.Hist = metric.Histogram()
	// 	meta.temporality = data.Hist.AggregationTemporality()
	// 	meta.monotonic = true
	// 	metric.SetEmptyHistogram()
	// case pmetric.MetricTypeExponentialHistogram:
	// 	data.Exp = metric.ExponentialHistogram()
	// 	meta.temporality = data.Exp.AggregationTemporality()
	// 	meta.monotonic = true
	// 	metric.SetEmptyExponentialHistogram()
	// }

	// return Metric{Meta: meta, Data: data}
}
