// Copyright The OpenTelemetry Authors
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

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
)

func initMeter() api.Meter {
	exporter, err := prometheus.New()
	if err != nil {
		log.Panicf("failed to initialize prometheus exporter %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", promhttp.Handler())
	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 20 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Panicf("failed to start prometheus server %v", err)
		}
	}()
	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	return provider.Meter("federation/prom-counter")
}

func main() {
	// set up prometheus
	meter := initMeter()
	// logging
	logger, _ := zap.NewProduction()
	defer func() {
		_ = logger.Sync()
	}()
	logger.Info("Start Prometheus metrics app")
	valueRecorder, err := meter.Int64Histogram("prom_counter")
	if err != nil {
		log.Panicf("failed to initialize histogram %v", err)
	}
	ctx := context.Background()
	valueRecorder.Record(ctx, 0)
	commonLabels := []attribute.KeyValue{attribute.String("A", "1"), attribute.String("B", "2"), attribute.String("C", "3")}
	counter := int64(0)
	valueRecorder.Record(ctx, counter, api.WithAttributes(commonLabels...))
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			time.Sleep(1 * time.Second)
			counter++
			valueRecorder.Record(ctx, counter, api.WithAttributes(commonLabels...))
			break
		case <-c:
			ticker.Stop()
			logger.Info("Stop Prometheus metrics app")
			return
		}
	}
}
