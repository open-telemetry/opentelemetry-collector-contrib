// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows && !darwin
// +build !linux,!windows,!darwin

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"
)

func (rmb *ResourceMetricsBuilder) RecordCPUTimeMetric(now pcommon.Timestamp, cpuTime *cpu.TimesStat) {
}

func (rmb *ResourceMetricsBuilder) RecordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
}
