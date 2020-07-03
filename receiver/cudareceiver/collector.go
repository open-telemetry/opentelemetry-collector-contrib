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

package cudareceiver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"contrib.go.opencensus.io/resource/auto"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/golang/protobuf/ptypes/timestamp"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
)

var (
	rsc                   *resourcepb.Resource
	resourceDetectionSync sync.Once
)

// CUDAMetricsCollector is a struct that collects and reports CUDA (GPU) metrics (temprature, power, et al.).
//
// TODO: ConsumeMetricsOld will be deprecated and should be replaced with ConsumerMetrics.
type CUDAMetricsCollector struct {
	consumer consumer.MetricsConsumerOld

	startTime time.Time
	device    *NVMLDevice

	scrapeInterval time.Duration
	metricPrefix   string
	done           chan struct{}
}

// NewCUDAMetricsCollector creates a new set of CUDA (GPU) Metrics (temperature, power, et al.).
func NewCUDAMetricsCollector(d time.Duration, prefix string, consumer consumer.MetricsConsumerOld) (*CUDAMetricsCollector, error) {
	device, status := NVMLDeviceGetHandledByIndex(uint64(0))
	if status != NVMLSuccess {
		return nil, fmt.Errorf("Could not get GPU device: status=%d", status)
	}

	c := &CUDAMetricsCollector{
		consumer:       consumer,
		startTime:      time.Now(),
		device:         device,
		scrapeInterval: d,
		metricPrefix:   prefix,
		done:           make(chan struct{}),
	}

	return c, nil
}

func detectResource() {
	resourceDetectionSync.Do(func() {
		res, err := auto.Detect(context.Background())
		if err != nil {
			panic(fmt.Sprintf("Resource detection failed, err:%v", err))
		}
		if res != nil {
			rsc = &resourcepb.Resource{
				Type:   res.Type,
				Labels: make(map[string]string, len(res.Labels)),
			}
			for k, v := range res.Labels {
				rsc.Labels[k] = v
			}
		}
	})
}

// StartCollection starts a ticker'd goroutine that will scrape and export CUDA metrics periodically.
func (c *CUDAMetricsCollector) StartCollection() {
	detectResource()
	status := NVMLInit()
	if status != NVMLSuccess {
		// TODO: handle inappropriate GPU state
		return
	}

	go func() {
		ticker := time.NewTicker(c.scrapeInterval)
		for {
			select {
			case <-ticker.C:
				c.scrapeAndExport()

			case <-c.done:
				return
			}
		}
	}()
}

// StopCollection stops the collection of metric information
func (c *CUDAMetricsCollector) StopCollection() {
	status := NVMLShutdown()
	if status != NVMLSuccess {
		// TODO: handle inappropriate GPU state
		return
	}
	close(c.done)
}

// scrapeAndExport
func (c *CUDAMetricsCollector) scrapeAndExport() {
	ctx := context.Background()

	metrics := make([]*metricspb.Metric, 0, len(cudaMetricDescriptors))
	var errs []error

	metrics = append(
		metrics,
		&metricspb.Metric{
			MetricDescriptor: metricTemperature,
			Resource:         rsc,
			Timeseries:       []*metricspb.TimeSeries{c.getInt64TimeSeries(c.device.Temperature())},
		},
		&metricspb.Metric{
			MetricDescriptor: metricPower,
			Resource:         rsc,
			Timeseries:       []*metricspb.TimeSeries{c.getInt64TimeSeries(c.device.PowerUsage())},
		},
		&metricspb.Metric{
			MetricDescriptor: metricPCIeThroughputTX,
			Resource:         rsc,
			Timeseries:       []*metricspb.TimeSeries{c.getInt64TimeSeries(c.device.PCIeThroughput(PCIeUtilTXBytes))},
		},
		&metricspb.Metric{
			MetricDescriptor: metricPCIeThroughputRX,
			Resource:         rsc,
			Timeseries:       []*metricspb.TimeSeries{c.getInt64TimeSeries(c.device.PCIeThroughput(PCIeUtilRXBytes))},
		},
	)

	if len(errs) > 0 {
		// TODO: emit error log
		return
	}

	c.consumer.ConsumeMetricsData(ctx, consumerdata.MetricsData{Metrics: metrics})
}

// TimeToTimestamp converts a time.Time to a timestamp.Timestamp pointer.
// TODO: remove this function once it gets exposed.
// https://github.com/open-telemetry/opentelemetry-collector/blob/master/internal/internal.go
func TimeToTimestamp(t time.Time) *timestamp.Timestamp {
	if t.IsZero() {
		return nil
	}
	nanoTime := t.UnixNano()
	return &timestamp.Timestamp{
		Seconds: nanoTime / 1e9,
		Nanos:   int32(nanoTime % 1e9),
	}
}

func (c *CUDAMetricsCollector) getInt64TimeSeries(val uint64) *metricspb.TimeSeries {
	return &metricspb.TimeSeries{
		StartTimestamp: TimeToTimestamp(c.startTime),
		Points:         []*metricspb.Point{{Timestamp: TimeToTimestamp(time.Now()), Value: &metricspb.Point_Int64Value{Int64Value: int64(val)}}},
	}
}
