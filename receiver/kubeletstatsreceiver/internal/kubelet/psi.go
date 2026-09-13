// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

// addPSIMetrics records PSI avg and total data points for a single PSIStats value.
// It is nil-safe: if psiStats is nil (e.g. on cgroup v1 or Windows nodes) it is a no-op.
// PSIData.Total is in nanoseconds, matching the unit: ns declaration in metadata.yaml.
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
		psiType metadata.AttributePressureType
	}{
		{s.Some, metadata.AttributePressureTypeSome},
		{s.Full, metadata.AttributePressureTypeFull},
	}
	for _, entry := range entries {
		m.Total(mb, currentTime, int64(entry.data.Total), entry.psiType) //nolint:gosec // G115: uint64→int64 safe; overflow would require 292+ years of cumulative stall. Kernel counters reset on reboot.
		m.Avg(mb, currentTime, entry.data.Avg10, entry.psiType, metadata.AttributePressureWindow10s)
		m.Avg(mb, currentTime, entry.data.Avg60, entry.psiType, metadata.AttributePressureWindow60s)
		m.Avg(mb, currentTime, entry.data.Avg300, entry.psiType, metadata.AttributePressureWindow300s)
	}
}
