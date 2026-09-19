// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

// addPSIMetrics records PSI avg and time data points for a single PSIStats value.
// It is nil-safe: if psiStats is nil (e.g. on cgroup v1 or Windows nodes) it is a no-op.
// PSIData.Total is in nanoseconds from the kubelet API; we convert to seconds (unit: s in metadata.yaml).
func addPSIMetrics(
	mb *metadata.MetricsBuilder,
	m metadata.PSIMetrics,
	s *stats.PSIStats,
	currentTime pcommon.Timestamp,
) {
	if s == nil {
		return
	}
	entries := [2]struct {
		data    stats.PSIData
		psiType metadata.AttributePsiType
	}{
		{s.Some, metadata.AttributePsiTypeSome},
		{s.Full, metadata.AttributePsiTypeFull},
	}
	for _, entry := range entries {
		m.Time(mb, currentTime, float64(entry.data.Total)/1e9, entry.psiType) // Convert nanoseconds → seconds
		m.Avg(mb, currentTime, entry.data.Avg10, entry.psiType, metadata.AttributePsiWindow10s)
		m.Avg(mb, currentTime, entry.data.Avg60, entry.psiType, metadata.AttributePsiWindow60s)
		m.Avg(mb, currentTime, entry.data.Avg300, entry.psiType, metadata.AttributePsiWindow300s)
	}
}
