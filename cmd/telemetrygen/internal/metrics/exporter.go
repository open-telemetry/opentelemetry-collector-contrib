// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc"
)

// NewGRPCExporter creates and starts a gRPC-based OTLP metric exporter.
// It configures the exporter with the provided endpoint, connection security settings, and headers.
func NewGRPCExporter(cfg *Config) (metric.Exporter, error) {
	grpcExpOpt := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint()),
		otlpmetricgrpc.WithDialOption(
			grpc.WithBlock(),
		),
	}

	if cfg.Insecure {
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithInsecure())
	}

	if len(cfg.Headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithHeaders(cfg.Headers))
	}

	return otlpmetricgrpc.New(context.Background(), grpcExpOpt...)
}

// NewHTTPExporter creates and starts an HTTP-based OTLP metric exporter.
// It configures the exporter with the provided endpoint, URL path, connection security settings, and headers.
func NewHTTPExporter(cfg *Config) (metric.Exporter, error) {
	httpExpOpt := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.Endpoint()),
		otlpmetrichttp.WithURLPath(cfg.HTTPPath),
	}

	if cfg.Insecure {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithInsecure())
	}

	if len(cfg.Headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithHeaders(cfg.Headers))
	}

	return otlpmetrichttp.New(context.Background(), httpExpOpt...)
}
