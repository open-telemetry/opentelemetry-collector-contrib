// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin
// +build linux darwin

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"
)

func (rmb *ResourceMetricsBuilder) RecordCPUTimeMetric(now pcommon.Timestamp, cpuTime *cpu.TimesStat) {
	rmb.RecordProcessCPUTimeDataPoint(now, cpuTime.User, AttributeStateUser)
	rmb.RecordProcessCPUTimeDataPoint(now, cpuTime.System, AttributeStateSystem)
	rmb.RecordProcessCPUTimeDataPoint(now, cpuTime.Iowait, AttributeStateWait)
}

func (rmb *ResourceMetricsBuilder) RecordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
	rmb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.User, AttributeStateUser)
	rmb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.System, AttributeStateSystem)
	rmb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.Iowait, AttributeStateWait)
}
