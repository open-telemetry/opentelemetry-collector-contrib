// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"

	grpcZap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/tracegen/internal/tracegen"
)

func main() {
	fs := flag.CommandLine
	cfg := new(tracegen.Config)
	cfg.Flags(fs)
	flag.Parse()

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("failed to obtain logger: %v", err))
	}
	grpcZap.ReplaceGrpcLoggerV2(logger.WithOptions(
		zap.AddCallerSkip(3),
	))

	grpcExpOpt := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithDialOption(
			grpc.WithBlock(),
		),
	}

	httpExpOpt := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint),
	}

	if cfg.Insecure {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithInsecure())
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithInsecure())
	}

	if len(cfg.Headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithHeaders(cfg.Headers))
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithHeaders(cfg.Headers))
	}

	var exp *otlptrace.Exporter
	if cfg.UseHTTP {
		logger.Info("starting HTTP exporter")
		exp, err = otlptracehttp.New(context.Background(), httpExpOpt...)
	} else {
		logger.Info("starting gRPC exporter")
		exp, err = otlptracegrpc.New(context.Background(), grpcExpOpt...)
	}

	if err != nil {
		logger.Error("failed to obtain OTLP exporter", zap.Error(err))
		return
	}
	defer func() {
		logger.Info("stopping the exporter")
		if err = exp.Shutdown(context.Background()); err != nil {
			logger.Error("failed to stop the exporter", zap.Error(err))
			return
		}
	}()

	ssp := sdktrace.NewBatchSpanProcessor(exp, sdktrace.WithBatchTimeout(time.Second))
	defer func() {
		logger.Info("stop the batch span processor")
		if err := ssp.Shutdown(context.Background()); err != nil {
			logger.Error("failed to stop the batch span processor", zap.Error(err))
			return
		}
	}()

	var attributes []attribute.KeyValue
	// may be overridden by `-otlp-attributes service.name="foo"`
	attributes = append(attributes, semconv.ServiceNameKey.String(cfg.ServiceName))

	if len(cfg.ResourceAttributes) > 0 {
		for k, v := range cfg.ResourceAttributes {
			attributes = append(attributes, attribute.String(k, v))
		}
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, attributes...)),
	)

	tracerProvider.RegisterSpanProcessor(ssp)
	otel.SetTracerProvider(tracerProvider)

	if err := tracegen.Run(cfg, logger); err != nil {
		logger.Error("failed to stop the exporter", zap.Error(err))
	}
}
