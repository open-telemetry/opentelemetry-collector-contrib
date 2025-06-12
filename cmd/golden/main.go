// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/golden"

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/golden/internal"
)

// insertDefault is a helper function to insert a default value for a configoptional.Optional type.
func insertDefault[T any](opt *configoptional.Optional[T]) error {
	if opt.HasValue() {
		return nil
	}

	empty := confmap.NewFromStringMap(map[string]any{})
	return empty.Unmarshal(opt)
}

func main() {
	if err := run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	cfg, err := internal.ReadConfig(args)
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
		if insertErr := insertDefault(&receiverConfig.HTTP); insertErr != nil {
			return insertErr
		}
		receiverConfig.HTTP.Get().ServerConfig.Endpoint = cfg.OTLPHTTPEndoint
	}

	if cfg.OTLPEndpoint != "" {
		if insertErr := insertDefault(&receiverConfig.GRPC); insertErr != nil {
			return insertErr
		}
		receiverConfig.GRPC.Get().NetAddr.Endpoint = cfg.OTLPEndpoint
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
	timeout := false
	select {
	case <-ticker.C:
		logger.Error("Timeout waiting for data")
		timeout = true
	case <-sink.DoneChan:
	}

	if sink.Error != nil {
		return sink.Error
	}
	if timeout {
		return errors.New("timeout waiting for data")
	}

	return nil
}
