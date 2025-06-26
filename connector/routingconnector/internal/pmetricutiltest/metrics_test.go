// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricutiltest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pmetricutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestNewMetrics(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		expected := pmetric.NewMetrics()
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewGauges("", "", "", "")))
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewSums("", "", "", "")))
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewHistograms("", "", "", "")))
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewExponentialHistograms("", "", "", "")))
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewSummaries("", "", "", "")))
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts()))
	})

	t.Run("simple", func(t *testing.T) {
		t.Run("gauge", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptyGauge()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpD") // resourceA.scopeB.metricC.dpD
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewGauges("A", "B", "C", "D")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B", pmetricutiltest.Gauge("C", pmetricutiltest.NumberDataPoint("D"))),
				),
			)))
		})
		t.Run("sum", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptySum()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpD") // resourceA.scopeB.metricC.dpD
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewSums("A", "B", "C", "D")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B", pmetricutiltest.Sum("C", pmetricutiltest.NumberDataPoint("D"))),
				),
			)))
		})
		t.Run("histogram", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptyHistogram()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpD") // resourceA.scopeB.metricC.dpD
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewHistograms("A", "B", "C", "D")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B", pmetricutiltest.Histogram("C", pmetricutiltest.HistogramDataPoint("D"))),
				),
			)))
		})
		t.Run("exponential_histogram", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptyExponentialHistogram()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpD") // resourceA.scopeB.metricC.dpD
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewExponentialHistograms("A", "B", "C", "D")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B", pmetricutiltest.ExponentialHistogram("C", pmetricutiltest.ExponentialHistogramDataPoint("D"))),
				),
			)))
		})
		t.Run("summary", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptySummary()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpD") // resourceA.scopeB.metricC.dpD
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewSummaries("A", "B", "C", "D")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B", pmetricutiltest.Summary("C", pmetricutiltest.SummaryDataPoint("D"))),
				),
			)))
		})
	})

	t.Run("two_resources", func(t *testing.T) {
		expected := func() pmetric.Metrics {
			md := pmetric.NewMetrics()
			r := md.ResourceMetrics().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			s := r.ScopeMetrics().AppendEmpty()
			s.Scope().SetName("scopeC") // resourceA.scopeC
			m := s.Metrics().AppendEmpty()
			m.SetName("metricD") // resourceA.scopeC.metricD
			dps := m.SetEmptyGauge()
			dp := dps.DataPoints().AppendEmpty()
			dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeC.metricD.dpE
			r = md.ResourceMetrics().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceB") // resourceB
			s = r.ScopeMetrics().AppendEmpty()
			s.Scope().SetName("scopeC") // resourceB.scopeC
			m = s.Metrics().AppendEmpty()
			m.SetName("metricD") // resourceB.scopeC.metricD
			dps = m.SetEmptyGauge()
			dp = dps.DataPoints().AppendEmpty()
			dp.Attributes().PutStr("dpName", "dpE") // resourceB.scopeC.metricD.dpE
			return md
		}()
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewGauges("AB", "C", "D", "E")))
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
			pmetricutiltest.Resource("A",
				pmetricutiltest.Scope("C", pmetricutiltest.Gauge("D", pmetricutiltest.NumberDataPoint("E"))),
			),
			pmetricutiltest.Resource("B",
				pmetricutiltest.Scope("C", pmetricutiltest.Gauge("D", pmetricutiltest.NumberDataPoint("E"))),
			),
		)))
	})

	t.Run("two_scopes", func(t *testing.T) {
		expected := func() pmetric.Metrics {
			md := pmetric.NewMetrics()
			r := md.ResourceMetrics().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			s := r.ScopeMetrics().AppendEmpty()
			s.Scope().SetName("scopeB") // resourceA.scopeB
			m := s.Metrics().AppendEmpty()
			m.SetName("metricD") // resourceA.scopeB.metricD
			dps := m.SetEmptyGauge()
			dp := dps.DataPoints().AppendEmpty()
			dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricD.dpE
			s = r.ScopeMetrics().AppendEmpty()
			s.Scope().SetName("scopeC") // resourceA.scopeC
			m = s.Metrics().AppendEmpty()
			m.SetName("metricD") // resourceA.scopeC.metricD
			dps = m.SetEmptyGauge()
			dp = dps.DataPoints().AppendEmpty()
			dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeC.metricD.dpE
			return md
		}()
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewGauges("A", "BC", "D", "E")))
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
			pmetricutiltest.Resource("A",
				pmetricutiltest.Scope("B", pmetricutiltest.Gauge("D", pmetricutiltest.NumberDataPoint("E"))),
				pmetricutiltest.Scope("C", pmetricutiltest.Gauge("D", pmetricutiltest.NumberDataPoint("E"))),
			),
		)))
	})

	t.Run("two_metrics", func(t *testing.T) {
		t.Run("gauges", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptyGauge()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricC.dpE
				m = s.Metrics().AppendEmpty()
				m.SetName("metricD") // resourceA.scopeB.metricD
				dps = m.SetEmptyGauge()
				dp = dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricD.dpE
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewGauges("A", "B", "CD", "E")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B",
						pmetricutiltest.Gauge("C", pmetricutiltest.NumberDataPoint("E")),
						pmetricutiltest.Gauge("D", pmetricutiltest.NumberDataPoint("E")),
					),
				),
			)))
		})
		t.Run("sums", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptySum()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricC.dpE
				m = s.Metrics().AppendEmpty()
				m.SetName("metricD") // resourceA.scopeB.metricD
				dps = m.SetEmptySum()
				dp = dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricD.dpE
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewSums("A", "B", "CD", "E")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B",
						pmetricutiltest.Sum("C", pmetricutiltest.NumberDataPoint("E")),
						pmetricutiltest.Sum("D", pmetricutiltest.NumberDataPoint("E")),
					),
				),
			)))
		})
		t.Run("histograms", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptyHistogram()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricC.dpE
				m = s.Metrics().AppendEmpty()
				m.SetName("metricD") // resourceA.scopeB.metricD
				dps = m.SetEmptyHistogram()
				dp = dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricD.dpE
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewHistograms("A", "B", "CD", "E")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B",
						pmetricutiltest.Histogram("C", pmetricutiltest.HistogramDataPoint("E")),
						pmetricutiltest.Histogram("D", pmetricutiltest.HistogramDataPoint("E")),
					),
				),
			)))
		})
		t.Run("exponential_histograms", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptyExponentialHistogram()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricC.dpE
				m = s.Metrics().AppendEmpty()
				m.SetName("metricD") // resourceA.scopeB.metricD
				dps = m.SetEmptyExponentialHistogram()
				dp = dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricD.dpE
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewExponentialHistograms("A", "B", "CD", "E")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B",
						pmetricutiltest.ExponentialHistogram("C", pmetricutiltest.ExponentialHistogramDataPoint("E")),
						pmetricutiltest.ExponentialHistogram("D", pmetricutiltest.ExponentialHistogramDataPoint("E")),
					),
				),
			)))
		})
		t.Run("summaries", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptySummary()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricC.dpE
				m = s.Metrics().AppendEmpty()
				m.SetName("metricD") // resourceA.scopeB.metricD
				dps = m.SetEmptySummary()
				dp = dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricD.dpE
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewSummaries("A", "B", "CD", "E")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B",
						pmetricutiltest.Summary("C", pmetricutiltest.SummaryDataPoint("E")),
						pmetricutiltest.Summary("D", pmetricutiltest.SummaryDataPoint("E")),
					),
				),
			)))
		})
	})

	t.Run("two_datapoints", func(t *testing.T) {
		t.Run("gauge", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptyGauge()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpD") // resourceA.scopeB.metricC.dpD
				dp = dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricC.dpE
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewGauges("A", "B", "C", "DE")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B", pmetricutiltest.Gauge("C", pmetricutiltest.NumberDataPoint("D"), pmetricutiltest.NumberDataPoint("E"))),
				),
			)))
		})
		t.Run("sum", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptySum()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpD") // resourceA.scopeB.metricC.dpD
				dp = dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricC.dpE
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewSums("A", "B", "C", "DE")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B", pmetricutiltest.Sum("C", pmetricutiltest.NumberDataPoint("D"), pmetricutiltest.NumberDataPoint("E"))),
				),
			)))
		})
		t.Run("histogram", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptyHistogram()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpD") // resourceA.scopeB.metricC.dpD
				dp = dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricC.dpE
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewHistograms("A", "B", "C", "DE")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B", pmetricutiltest.Histogram("C", pmetricutiltest.HistogramDataPoint("D"), pmetricutiltest.HistogramDataPoint("E"))),
				),
			)))
		})
		t.Run("exponential_histogram", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptyExponentialHistogram()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpD") // resourceA.scopeB.metricC.dpD
				dp = dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricC.dpE
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewExponentialHistograms("A", "B", "C", "DE")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B", pmetricutiltest.ExponentialHistogram("C", pmetricutiltest.ExponentialHistogramDataPoint("D"), pmetricutiltest.ExponentialHistogramDataPoint("E"))),
				),
			)))
		})
		t.Run("summary", func(t *testing.T) {
			expected := func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				r := md.ResourceMetrics().AppendEmpty()
				r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
				s := r.ScopeMetrics().AppendEmpty()
				s.Scope().SetName("scopeB") // resourceA.scopeB
				m := s.Metrics().AppendEmpty()
				m.SetName("metricC") // resourceA.scopeB.metricC
				dps := m.SetEmptySummary()
				dp := dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpD") // resourceA.scopeB.metricC.dpD
				dp = dps.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("dpName", "dpE") // resourceA.scopeB.metricC.dpE
				return md
			}()
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewSummaries("A", "B", "C", "DE")))
			assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("B", pmetricutiltest.Summary("C", pmetricutiltest.SummaryDataPoint("D"), pmetricutiltest.SummaryDataPoint("E"))),
				),
			)))
		})
	})

	t.Run("all_metric_types", func(t *testing.T) {
		expected := func() pmetric.Metrics {
			md := pmetric.NewMetrics()
			r := md.ResourceMetrics().AppendEmpty()
			r.Resource().Attributes().PutStr("resourceName", "resourceA") // resourceA
			s := r.ScopeMetrics().AppendEmpty()
			s.Scope().SetName("scopeB") // resourceA.scopeB

			m := s.Metrics().AppendEmpty()
			m.SetName("metricC") // resourceA.scopeB.metricC
			gauge := m.SetEmptyGauge()
			ndp := gauge.DataPoints().AppendEmpty()
			ndp.Attributes().PutStr("dpName", "dpX") // resourceA.scopeB.metricC.dpX
			ndp = gauge.DataPoints().AppendEmpty()
			ndp.Attributes().PutStr("dpName", "dpY") // resourceA.scopeB.metricC.dpY

			m = s.Metrics().AppendEmpty()
			m.SetName("metricD") // resourceA.scopeB.metricD
			sum := m.SetEmptySum()
			ndp = sum.DataPoints().AppendEmpty()
			ndp.Attributes().PutStr("dpName", "dpX") // resourceA.scopeB.metricD.dpX
			ndp = sum.DataPoints().AppendEmpty()
			ndp.Attributes().PutStr("dpName", "dpY") // resourceA.scopeB.metricD.dpY

			m = s.Metrics().AppendEmpty()
			m.SetName("metricE") // resourceA.scopeB.metricE
			hist := m.SetEmptyHistogram()
			hdp := hist.DataPoints().AppendEmpty()
			hdp.Attributes().PutStr("dpName", "dpX") // resourceA.scopeB.metricE.dpX
			hdp = hist.DataPoints().AppendEmpty()
			hdp.Attributes().PutStr("dpName", "dpY") // resourceA.scopeB.metricE.dpY

			m = s.Metrics().AppendEmpty()
			m.SetName("metricF") // resourceA.scopeB.metricF
			expHist := m.SetEmptyExponentialHistogram()
			edp := expHist.DataPoints().AppendEmpty()
			edp.Attributes().PutStr("dpName", "dpX") // resourceA.scopeB.metricF.dpX
			edp = expHist.DataPoints().AppendEmpty()
			edp.Attributes().PutStr("dpName", "dpY") // resourceA.scopeB.metricF.dpY

			m = s.Metrics().AppendEmpty()
			m.SetName("metricG") // resourceA.scopeB.metricG
			smry := m.SetEmptySummary()
			sdp := smry.DataPoints().AppendEmpty()
			sdp.Attributes().PutStr("dpName", "dpX") // resourceA.scopeB.metricG.dpX
			sdp = smry.DataPoints().AppendEmpty()
			sdp.Attributes().PutStr("dpName", "dpY") // resourceA.scopeB.metricG.dpY

			return md
		}()
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetricsFromOpts(
			pmetricutiltest.Resource("A",
				pmetricutiltest.Scope("B",
					pmetricutiltest.Gauge("C", pmetricutiltest.NumberDataPoint("X"), pmetricutiltest.NumberDataPoint("Y")),
					pmetricutiltest.Sum("D", pmetricutiltest.NumberDataPoint("X"), pmetricutiltest.NumberDataPoint("Y")),
					pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("X"), pmetricutiltest.HistogramDataPoint("Y")),
					pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("X"), pmetricutiltest.ExponentialHistogramDataPoint("Y")),
					pmetricutiltest.Summary("G", pmetricutiltest.SummaryDataPoint("X"), pmetricutiltest.SummaryDataPoint("Y")),
				),
			),
		)))
	})
}
