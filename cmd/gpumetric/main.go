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

package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// GPU metrics declarations
var (
	gpuTemp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gpu_temperature_celsius",
		Help: "Current temperature of the GPU.",
	})
	powerUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gpu_power_usage",
		Help: "Current power usage of the GPU.",
	})
	gpuPCIeThroughputRxBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gpu_pcie_throughput_rx_bytes",
		Help: "PCIe utilization counters of Rx in Bytes",
	})
	gpuPCIeThroughputTxBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gpu_pcie_throughput_tx_bytes",
		Help: "PCIe utilization counters of Tx in Bytes",
	})
	gpuPCIeThroughputCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gpu_pcie_throughput_count",
		Help: "PCIe utilization counters of Tx in Bytes",
	})
)

var (
	errGPUNotFound = errors.New("GPU was not found")
)

func init() {
	prometheus.MustRegister(gpuTemp)
	prometheus.MustRegister(powerUsage)
	prometheus.MustRegister(gpuPCIeThroughputRxBytes)
	prometheus.MustRegister(gpuPCIeThroughputTxBytes)
	prometheus.MustRegister(gpuPCIeThroughputCount)
}

func main() {
	d, err := newGPUDevice()
	if err != nil {
		log.Fatalf("failed to get a GPU device: %s", err)
		return
	}
	d.StartScraping()
	defer d.StopScraping()
	// TODO(ymotongpoo): Change endpoint to be configurable
	http.Handle("/metrics", promhttp.Handler())
	// TODO(ymotongpoo): Change port to be configurable
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("failed to serve metrics server", err)
	}
}

type device struct {
	d              *NVMLDevice
	scrapeInterval time.Duration
	done           chan struct{}
}

func newGPUDevice() (*device, error) {
	status := NVMLInit()
	if status != NVMLSuccess {
		log.Printf("could not initialize GPU device: status=%d", status)
		return nil, errGPUNotFound
	}
	d, status := NVMLDeviceGetHandledByIndex(uint64(0))
	if status != NVMLSuccess {
		log.Printf("could not get GPU device: status=%d", status)
		return nil, errGPUNotFound
	}
	return &device{
		d:              d,
		scrapeInterval: 5 * time.Second,
		done:           make(chan struct{}),
	}, nil
}

func (d *device) StartScraping() {
	status := NVMLInit()
	if status != NVMLSuccess {
		log.Printf("could not get GPU device: status=%d", status)
		return
	}
	go func() {
		ticker := time.NewTicker(d.scrapeInterval)
		for {
			select {
			case <-ticker.C:
				d.scrapeAndExport()

			case <-d.done:
				return
			}
		}
	}()
}

func (d *device) StopScraping() {
	status := NVMLShutdown()
	if status != NVMLSuccess {
		log.Printf("could shutdown GPU device: status=%d", status)
		return
	}
	close(d.done)
}

// scrapeAndExport updates all package global metrics.
func (d *device) scrapeAndExport() {
	temp := d.d.Temperature()
	power := d.d.PowerUsage()
	pcieTx := d.d.PCIeThroughput(PCIeUtilTXBytes)
	pcieRx := d.d.PCIeThroughput(PCIeUtilTXBytes)
	pcieCount := d.d.PCIeThroughput(PCIeUtilCount)

	gpuTemp.Set(float64(temp))
	powerUsage.Set(float64(power))
	gpuPCIeThroughputRxBytes.Set(float64(pcieTx))
	gpuPCIeThroughputTxBytes.Set(float64(pcieRx))
	gpuPCIeThroughputCount.Add(float64(pcieCount))
}
