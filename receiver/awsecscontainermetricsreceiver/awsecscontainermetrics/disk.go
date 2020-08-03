// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecscontainermetrics

import (
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

func diskMetrics(prefix string, s *DiskStats) []*metricspb.Metric {
	var readBytes *uint64
	var writeBytes *uint64

	for _, blockStat := range s.IoServiceBytesRecursives {
		switch op := blockStat.Op; op {
		case "Read":
			readBytes = blockStat.Value
		case "Write":
			writeBytes = blockStat.Value
		default:
			//ignoring "Async", "Total", "Sum", etc
			continue
		}
	}
	return applyCurrentTime([]*metricspb.Metric{
		storageReadBytes(prefix, readBytes),
		storageWriteBytes(prefix, writeBytes),
	}, time.Now())
}

func storageReadBytes(prefix string, value *uint64) *metricspb.Metric {
	return intGauge(prefix+"disk.storage_read_bytes", "Bytes", value)
}

func storageWriteBytes(prefix string, value *uint64) *metricspb.Metric {
	return intGauge(prefix+"disk.storage_write_bytes", "Bytes", value)
}
