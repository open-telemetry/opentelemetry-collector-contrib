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
	"context"
	"errors"
	"log"
	"time"

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	"go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

var (
	errGPUNotFound = errors.New("GPU was not found")
)

// GPU metrics declarations
var (
	temperature           metric.Int64ValueRecorder
	powerUsage            metric.Int64ValueRecorder
	pcieThroughputRxBytes metric.Int64ValueRecorder
	pcieThroughputTxBytes metric.Int64ValueRecorder
	pcieThroughputCount   metric.Int64ValueRecorder
)

func main() {
	d, err := newGPUDevice()
	if err != nil {
		log.Fatalf("failed to get a GPU device: %s", err)
		return
	}

	exporter, err := otlp.NewExporter(otlp.WithInsecure())
	if err != nil {
		log.Fatalf("failed to create OpenTelemetry exporter: %v", err)
	}
	defer func() {
		err := exporter.Stop()
		if err != nil {
			log.Fatalf("failed to stop OpenTelemetry exporter: %v", err)
		}
	}()

	pusher := push.New(
		basic.New(simple.NewWithExactDistribution(), exporter),
		exporter,
		push.WithPeriod(2*time.Second),
	)
	global.SetMeterProvider(pusher.Provider())
	pusher.Start()
	defer pusher.Stop()
	meterInit(pusher.Provider())

	ctx := context.Background()
	d.StartScraping(ctx)
	defer d.StopScraping()
}

func meterInit(p metric.Provider) metric.Meter {
	meter := p.Meter("opentelemetry-collector-contrib/gpumetric")
	temperature = metric.Must(meter).NewInt64ValueRecorder("temperature")
	powerUsage = metric.Must(meter).NewInt64ValueRecorder("powerusage")
	pcieThroughputRxBytes = metric.Must(meter).NewInt64ValueRecorder("throughput.rx")
	pcieThroughputTxBytes = metric.Must(meter).NewInt64ValueRecorder("throughput.tx")
	pcieThroughputCount = metric.Must(meter).NewInt64ValueRecorder("throughput.count")
	return meter
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

func (d *device) StartScraping(ctx context.Context) {
	status := NVMLInit()
	if status != NVMLSuccess {
		log.Printf("could not get GPU device: status=%d", status)
		return
	}
	ticker := time.NewTicker(d.scrapeInterval)
	for {
		select {
		case <-ticker.C:
			d.scrapeAndExport(ctx)

		case <-d.done:
			return
		}
	}
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
func (d *device) scrapeAndExport(ctx context.Context) {
	temp := d.d.Temperature()
	pu := d.d.PowerUsage()
	pcieTx := d.d.PCIeThroughput(PCIeUtilTXBytes)
	pcieRx := d.d.PCIeThroughput(PCIeUtilTXBytes)
	pcieCount := d.d.PCIeThroughput(PCIeUtilCount)

	temperature.Record(ctx, int64(temp))
	powerUsage.Record(ctx, int64(pu))
	pcieThroughputRxBytes.Record(ctx, int64(pcieRx))
	pcieThroughputTxBytes.Record(ctx, int64(pcieTx))
	pcieThroughputCount.Record(ctx, int64(pcieCount))
}
