// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"

import "go.opentelemetry.io/collector/pdata/pcommon"

func (mb *MetricsBuilder) RecordVcenterResourcePoolMemoryUsageDataPointWithoutTypeAttribute(ts pcommon.Timestamp, val int64) {
	mb.metricVcenterResourcePoolMemoryUsage.recordDataPointWithoutType(ts, val)
}

func (m *metricVcenterResourcePoolMemoryUsage) recordDataPointWithoutType(ts pcommon.Timestamp, val int64) {
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
}
