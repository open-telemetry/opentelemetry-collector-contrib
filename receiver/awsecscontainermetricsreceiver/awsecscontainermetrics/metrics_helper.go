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
	"strconv"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
)

const (
	// BytesInMiB is the number of bytes in a MebiByte.
	BytesInMiB = 1024 * 1024
)

func getContainerMetrics(stats ContainerStats) ECSMetrics {
	memoryUtilizedInMb := (*stats.Memory.Usage - stats.Memory.Stats["cache"]) / BytesInMiB

	numOfCores := (uint64)(len(stats.CPU.CpuUsage.PerCpuUsage))
	cpuUtilized := (float64)(*stats.CPU.CpuUsage.TotalUsage / numOfCores / 1024)

	netStatArray := getNetworkStats(stats.Network)

	storageReadBytes, storageWriteBytes := extractStorageUsage(&stats.Disk)

	m := ECSMetrics{}

	m.MemoryUsage = *stats.Memory.Usage
	m.MemoryMaxUsage = *stats.Memory.MaxUsage
	m.MemoryLimit = *stats.Memory.Limit
	m.MemoryUtilized = memoryUtilizedInMb

	m.CPUTotalUsage = *stats.CPU.CpuUsage.TotalUsage
	m.CPUUsageInKernelmode = *stats.CPU.CpuUsage.UsageInKernelmode
	m.CPUUsageInUserMode = *stats.CPU.CpuUsage.UsageInUserMode
	m.NumOfCPUCores = numOfCores
	m.CPUOnlineCpus = *stats.CPU.OnlineCpus
	m.SystemCPUUsage = *stats.CPU.SystemCpuUsage
	m.CPUUtilized = cpuUtilized

	m.NetworkRateRxBytesPerSecond = *stats.NetworkRate.TxBytesPerSecond
	m.NetworkRateTxBytesPerSecond = *stats.NetworkRate.TxBytesPerSecond

	m.NetworkRxBytes = netStatArray[0]
	m.NetworkRxPackets = netStatArray[1]
	m.NetworkRxErrors = netStatArray[2]
	m.NetworkRxDropped = netStatArray[3]

	m.NetworkTxBytes = netStatArray[4]
	m.NetworkTxPackets = netStatArray[5]
	m.NetworkTxErrors = netStatArray[6]
	m.NetworkTxDropped = netStatArray[7]

	m.StorageReadBytes = storageReadBytes
	m.StorageWriteBytes = storageWriteBytes
	return m
}

func getNetworkStats(stats map[string]NetworkStats) [8]uint64 {
	var netStatArray [8]uint64
	for _, netStat := range stats {
		netStatArray[0] += *netStat.RxBytes
		netStatArray[1] += *netStat.RxPackets
		netStatArray[2] += *netStat.RxErrors
		netStatArray[3] += *netStat.RxDropped

		netStatArray[4] += *netStat.TxBytes
		netStatArray[5] += *netStat.TxPackets
		netStatArray[6] += *netStat.TxErrors
		netStatArray[7] += *netStat.TxDropped
	}
	return netStatArray
}

func extractStorageUsage(stats *DiskStats) (uint64, uint64) {
	var readBytes, writeBytes uint64
	for _, blockStat := range stats.IoServiceBytesRecursives {
		switch op := blockStat.Op; op {
		case "Read":
			readBytes = *blockStat.Value
		case "Write":
			writeBytes = *blockStat.Value
		default:
			//ignoring "Async", "Total", "Sum", etc
			continue
		}
	}
	return readBytes, writeBytes
}

// GenerateDummyMetrics generates some dummy metrics
func GenerateDummyMetrics() consumerdata.MetricsData {

	// URL := "https://jsonplaceholder.typicode.com/todos/1"
	// URL :=  os.Getenv("URL")

	// resp, err := http.Get(URL)
	// if err != nil {
	// 	panic(err)
	// }
	// defer resp.Body.Close()
	// body, err := ioutil.ReadAll(resp.Body)
	// fmt.Println("get:\n", string(body))

	md := consumerdata.MetricsData{}

	ts := time.Now()
	for i := 0; i < 5; i++ {
		md.Metrics = append(md.Metrics,
			metricstestutil.Gauge(
				"test_"+strconv.Itoa(i),
				[]string{"k0", "k1"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1"},
					&metricspb.Point{
						Timestamp: metricstestutil.Timestamp(ts),
						Value:     &metricspb.Point_Int64Value{Int64Value: int64(i)},
					},
				),
			),
		)
	}
	return md
}
