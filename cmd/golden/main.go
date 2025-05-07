// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/golden"

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/golden/internal"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := internal.ReadConfig(os.Args)
	if err != nil {
		return err
	}

	sink, err := internal.NewConsumer(cfg)
	if err != nil {
		return err
	}

	factory := otlpreceiver.NewFactory()
	receiverConfig := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	if cfg.OTLPHTTPEndoint != "" {
		receiverConfig.HTTP.ServerConfig.Endpoint = cfg.OTLPHTTPEndoint
	} else {
		receiverConfig.HTTP = nil
	}
	if cfg.OTLPEndpoint != "" {
		receiverConfig.GRPC.NetAddr.Endpoint = cfg.OTLPEndpoint
	} else {
		receiverConfig.GRPC = nil
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}

	set := receiver.Settings{
		ID: component.MustNewID(factory.Type().String()),
		TelemetrySettings: component.TelemetrySettings{
			Logger:         logger,
			MeterProvider:  noopmetric.NewMeterProvider(),
			TracerProvider: nooptrace.NewTracerProvider(),
		},
		BuildInfo: component.BuildInfo{},
	}

	otlpReceiver, err := factory.CreateMetrics(context.Background(), set, receiverConfig, sink)
	if err != nil {
		return err
	}

	err = otlpReceiver.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		return err
	}
	defer func() {
		_ = otlpReceiver.Shutdown(context.Background())
	}()

	ticker := time.NewTicker(cfg.Timeout)
	select {
	case <-ticker.C:
		logger.Error("Timeout waiting for data")
	case <-sink.DoneChan:
	}

	if sink.Error != nil {
		return sink.Error
	}

	return nil
}
