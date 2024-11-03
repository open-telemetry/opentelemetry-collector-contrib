// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricutiltest // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pmetricutiltest"

import "go.opentelemetry.io/collector/pdata/pmetric"

// TestMetrics returns a pmetric.Metrics with a uniform structure where resources, scopes, metrics,
// and datapoints are identical across all instances, except for one identifying field.
//
// Identifying fields:
// - Resources have an attribute called "resourceName" with a value of "resourceN".
// - Scopes have a name with a value of "scopeN".
// - Metrics have a name with a value of "metricN" and a single time series of data points.
// - DataPoints have an attribute "dpName" with a value of "dpN".
//
// Example: TestMetrics("AB", "XYZ", "MN", "1234") returns:
//
//	resourceA, resourceB
//	    each with scopeX, scopeY, scopeZ
//	        each with metricM, metricN
//	            each with dp1, dp2, dp3, dp4
//
// Each byte in the input string is a unique ID for the corresponding element.
func NewMetrics(rIDs, sIDs, mIDs, dpIDs string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for ri := 0; ri < len(rIDs); ri++ {
		r := md.ResourceMetrics().AppendEmpty()
		r.Resource().Attributes().PutStr("resourceName", "resource"+string(rIDs[ri]))
		for si := 0; si < len(sIDs); si++ {
			s := r.ScopeMetrics().AppendEmpty()
			s.Scope().SetName("scope" + string(sIDs[si]))
			for mi := 0; mi < len(mIDs); mi++ {
				m := s.Metrics().AppendEmpty()
				m.SetName("metric" + string(mIDs[mi]))
				dps := m.SetEmptyGauge()
				for di := 0; di < len(dpIDs); di++ {
					dp := dps.DataPoints().AppendEmpty()
					dp.Attributes().PutStr("dpName", "dp"+string(dpIDs[di]))
				}
			}
		}
	}
	return md
}
