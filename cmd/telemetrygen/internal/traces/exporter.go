// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// grpcExporterOptions creates the configuration options for a gRPC-based OTLP trace exporter.
// It configures the exporter with the provided endpoint, connection security settings, and headers.
func grpcExporterOptions(cfg *Config) ([]otlptracegrpc.Option, error) {
	grpcExpOpt := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint()),
	}

	if cfg.Insecure {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithInsecure())
	} else {
		credentials, err := common.GetTLSCredentialsForGRPCExporter(
			cfg.CaFile, cfg.ClientAuth, cfg.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithTLSCredentials(credentials))
	}

	if len(cfg.Headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithHeaders(cfg.GetHeaders()))
	}

	return grpcExpOpt, nil
}

// httpExporterOptions creates the configuration options for an HTTP-based OTLP trace exporter.
// It configures the exporter with the provided endpoint, URL path, connection security settings, and headers.
func httpExporterOptions(cfg *Config) ([]otlptracehttp.Option, error) {
	httpExpOpt := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint()),
		otlptracehttp.WithURLPath(cfg.HTTPPath),
	}

	if cfg.Insecure {
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithInsecure())
	} else {
		tlsCfg, err := common.GetTLSCredentialsForHTTPExporter(
			cfg.CaFile, cfg.ClientAuth, cfg.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithTLSClientConfig(tlsCfg))
	}

	if len(cfg.Headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithHeaders(cfg.GetHeaders()))
	}

	return httpExpOpt, nil
}
