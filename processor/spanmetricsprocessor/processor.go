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

package spanmetricsprocessor

import (
	"context"
	"math"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

var (
	maxDuration   = time.Duration(math.MaxInt64)
	maxDurationMs = float64(maxDuration.Milliseconds())

	defaultLatencyHistogramBucketsMs = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000, maxDurationMs,
	}
)

type processorImp struct {
	logger *zap.Logger
	config Config

	metricsExporter component.MetricsExporter
	nextConsumer    consumer.TracesConsumer

	// Additional dimensions to add to metrics.
	dimensions []Dimension

	// Call & Error counts.
	callSum map[string]int64

	// Latency histogram.
	latencyCount        map[string]uint64
	latencySum          map[string]float64
	latencyBucketCounts map[string][]uint64
	latencyBounds       []float64
}

func newProcessor(logger *zap.Logger, config configmodels.Exporter, nextConsumer consumer.TracesConsumer) (*processorImp, error) {
	logger.Info("building spanmetricsprocessor")
	pConfig := config.(*Config)

	bounds := defaultLatencyHistogramBucketsMs
	if pConfig.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(pConfig.LatencyHistogramBuckets, func(duration time.Duration) float64 {
			return float64(duration.Milliseconds())
		})

		// "Catch-all" bucket.
		if bounds[len(bounds)-1] != maxDurationMs {
			bounds = append(bounds, maxDurationMs)
		}
	}

	return &processorImp{
		logger:              logger,
		config:              *pConfig,
		callSum:             make(map[string]int64),
		latencyBounds:       bounds,
		latencySum:          make(map[string]float64),
		latencyCount:        make(map[string]uint64),
		latencyBucketCounts: make(map[string][]uint64),
		nextConsumer:        nextConsumer,
		dimensions:          pConfig.Dimensions,
	}, nil
}

func mapDurationsToMillis(vs []time.Duration, f func(duration time.Duration) float64) []float64 {
	vsm := make([]float64, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}

// Start implements the component.Component interface.
func (p *processorImp) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("starting spanmetricsprocessor")

	// TODO: Add implementation

	p.logger.Info("started spanmetricsprocessor")
	return nil
}

// Shutdown implements the component.Component interface.
func (p *processorImp) Shutdown(ctx context.Context) error {
	p.logger.Info("shutting down spanmetricsprocessor")
	return nil
}

// GetCapabilities implements the component.Processor interface.
func (p *processorImp) GetCapabilities() component.ProcessorCapabilities {
	p.logger.Info("GetCapabilities for spanmetricsprocessor")
	return component.ProcessorCapabilities{MutatesConsumedData: false}
}

// ConsumeTraces implements the consumer.TracesConsumer interface.
// It aggregates the trace data to generate metrics, forwarding these metrics
// to the discovered metrics exporter.
// The original input trace data will be forwarded to the next consumer, unmodified.
func (p *processorImp) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	p.logger.Info("consuming trace data")

	p.aggregateMetrics(traces)

	m := p.buildMetrics()

	// Firstly, export metrics to avoid being impacted by downstream trace processor errors/latency.
	if err := p.metricsExporter.ConsumeMetrics(ctx, *m); err != nil {
		return err
	}

	// Forward trace data unmodified.
	if err := p.nextConsumer.ConsumeTraces(ctx, traces); err != nil {
		return err
	}

	return nil
}

// buildMetrics collects the computed raw metrics data, builds the metrics object and
// writes the raw metrics data into the metrics object.
func (p *processorImp) buildMetrics() *pdata.Metrics {
	// TODO: Add implementation
	m := pdata.NewMetrics()
	return &m
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
// Each metric is identified by a key that is built from the service name
// and span metadata such as operation, kind, status_code and any additional
// dimensions the user has configured.
func (p *processorImp) aggregateMetrics(traces pdata.Traces) {
	// TODO: Add implementation
}
