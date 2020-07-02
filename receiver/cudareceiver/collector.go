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
	"os"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
)

// CUDAMetricsCollector is a struct that collects and reports CUDA (GPU) metrics (temprature, power, et al.).
//
// TODO: ConsumeMetricsOld will be deprecated and should be replaced with ConsumerMetrics.
type CUDAMetricsCollector struct {
	consumer consumer.MetricsConsumerOld

	startTime time.Time

	pid int

	scrapeInterval time.Duration
	metricPrefix   string
	done           chan struct{}
}

// NewCUDAMetricsCollector creates a new set of CUDA (GPU) Metrics (temperature, power, et al.).
func NewCUDAMetricsCollector(d time.Duration, prefix string, consumer consumer.MetricsConsumerOld) (*CUDAMetricsCollector, error) {
	c := &CUDAMetricsCollector{
		consumer:       consumer,
		startTime:      time.Now(),
		pid:            os.Getpid(),
		scrapeInterval: d,
		metricPrefix:   prefix,
		done:           make(chan struct{}),
	}

	return c, nil
}

// StartCollection starts a ticker'd goroutine that will scrape and export CUDA metrics periodically.
func (c *CUDAMetricsCollector) StartCollection() {
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
	close(c.done)
}

// scrapeAndExport
func (c *CUDAMetricsCollector) scrapeAndExport() {
	ctx := context.Background()

	metrics := make([]*metricspb.Metric, 0, len(cudaMetricDescriptors))
	var errs []error

	// TODO: fulfill metrics data using CUDA functions
	_ = metrics

	if len(errs) > 0 {
		// TODO: emit error log
		return
	}

	c.consumer.ConsumeMetricsData(ctx, consumerdata.MetricsData{Metrics: metrics})
}
