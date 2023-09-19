// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"google.golang.org/grpc"
)

func NewGRPCExporter(cfg *Config) (*otlptrace.Exporter, error) {
	grpcExpOpt := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithDialOption(
			grpc.WithBlock(),
		),
	}

	if cfg.Insecure {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithInsecure())
	}

	if len(cfg.Headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	return otlptracegrpc.New(context.Background(), grpcExpOpt...)
}

func NewHTTPExporter(cfg *Config) (*otlptrace.Exporter, error) {
	httpExpOpt := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithURLPath(cfg.HTTPPath),
	}

	if cfg.Insecure {
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithInsecure())
	}

	if len(cfg.Headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithHeaders(cfg.Headers))
	}

	return otlptracehttp.New(context.Background(), httpExpOpt...)
}
