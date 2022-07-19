// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux
// +build linux

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
)

func (s *scraper) recordCPUTimeStateDataPoints(now pcommon.Timestamp, cpuTime cpu.TimesStat) {
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.User, cpuTime.CPU, metadata.AttributeStateUser)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.System, cpuTime.CPU, metadata.AttributeStateSystem)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Idle, cpuTime.CPU, metadata.AttributeStateIdle)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Irq, cpuTime.CPU, metadata.AttributeStateInterrupt)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Nice, cpuTime.CPU, metadata.AttributeStateNice)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Softirq, cpuTime.CPU, metadata.AttributeStateSoftirq)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Steal, cpuTime.CPU, metadata.AttributeStateSteal)
	s.mb.RecordSystemCPUTimeDataPoint(now, cpuTime.Iowait, cpuTime.CPU, metadata.AttributeStateWait)
}

func (s *scraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.User, cpuUtilization.CPU, metadata.AttributeStateUser)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.System, cpuUtilization.CPU, metadata.AttributeStateSystem)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Idle, cpuUtilization.CPU, metadata.AttributeStateIdle)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Irq, cpuUtilization.CPU, metadata.AttributeStateInterrupt)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Nice, cpuUtilization.CPU, metadata.AttributeStateNice)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Softirq, cpuUtilization.CPU, metadata.AttributeStateSoftirq)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Steal, cpuUtilization.CPU, metadata.AttributeStateSteal)
	s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtilization.Iowait, cpuUtilization.CPU, metadata.AttributeStateWait)
}
