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
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetrics("", "", "", "")))
	})

	t.Run("simple", func(t *testing.T) {
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
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetrics("A", "B", "C", "D")))
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
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetrics("AB", "C", "D", "E")))
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
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetrics("A", "BC", "D", "E")))
	})

	t.Run("two_metrics", func(t *testing.T) {
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
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetrics("A", "B", "CD", "E")))
	})

	t.Run("two_datapoints", func(t *testing.T) {
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
		assert.NoError(t, pmetrictest.CompareMetrics(expected, pmetricutiltest.NewMetrics("A", "B", "C", "DE")))
	})
}
