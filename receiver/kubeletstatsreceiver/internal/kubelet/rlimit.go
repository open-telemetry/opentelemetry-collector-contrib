// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addRlimitMetrics(mb *metadata.MetricsBuilder, s *stats.RlimitStats, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}

	if s.MaxPID != nil {
		mb.RecordSystemProcessLimitDataPoint(currentTime, *s.MaxPID)
	}
	if s.NumOfRunningProcesses != nil {
		mb.RecordSystemProcessCountDataPoint(currentTime, *s.NumOfRunningProcesses, metadata.AttributeProcessStateRunning)
	}
}
