// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pmetricutil"

import "go.opentelemetry.io/collector/pdata/pmetric"

// MoveResourcesIf calls f sequentially for each ResourceSpans present in the first pmetric.Metrics.
// If f returns true, the element is removed from the first pmetric.Metrics and added to the second pmetric.Metrics.
func MoveResourcesIf(from, to pmetric.Metrics, f func(pmetric.ResourceMetrics) bool) {
	from.ResourceMetrics().RemoveIf(func(rs pmetric.ResourceMetrics) bool {
		if !f(rs) {
			return false
		}
		rs.CopyTo(to.ResourceMetrics().AppendEmpty())
		return true
	})
}
