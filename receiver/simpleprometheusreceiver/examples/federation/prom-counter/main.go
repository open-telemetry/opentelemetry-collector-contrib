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
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.uber.org/zap"
)

func initMeter() metric.Meter {
	config := prometheus.Config{}
	c := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries(config.DefaultHistogramBoundaries),
			),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
	)
	exporter, err := prometheus.New(config, c)

	if err != nil {
		log.Panicf("failed to initialize prometheus exporter %v", err)
	}
	http.HandleFunc("/", exporter.ServeHTTP)
	go func() {
		_ = http.ListenAndServe(":8080", nil)
	}()
	return exporter.MeterProvider().Meter("federation/prom-counter")
}

func main() {
	// set up prometheus
	meter := initMeter()
	// logging
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	logger.Info("Start Prometheus metrics app")
	valueRecorder, err := meter.SyncInt64().Histogram("prom_counter")
	if err != nil {
		log.Panicf("failed to initialize histogram %v", err)
	}
	ctx := context.Background()
	valueRecorder.Record(ctx, 0)
	commonLabels := []attribute.KeyValue{attribute.String("A", "1"), attribute.String("B", "2"), attribute.String("C", "3")}
	counter := int64(0)
	valueRecorder.Record(ctx, counter, commonLabels...)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			time.Sleep(1 * time.Second)
			counter++
			valueRecorder.Record(ctx, counter, commonLabels...)
			break
		case <-c:
			ticker.Stop()
			logger.Info("Stop Prometheus metrics app")
			return
		}
	}
}
