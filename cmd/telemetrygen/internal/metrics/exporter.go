// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// grpcExporterOptions creates the configuration options for a gRPC-based OTLP metric exporter.
// It configures the exporter with the provided endpoint, connection security settings, and headers.
func grpcExporterOptions(cfg *Config) ([]otlpmetricgrpc.Option, error) {
	grpcExpOpt := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint()),
	}

	if cfg.Insecure {
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithInsecure())
	} else {
		credentials, err := common.GetTLSCredentialsForGRPCExporter(
			cfg.CaFile, cfg.ClientAuth, cfg.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithTLSCredentials(credentials))
	}

	if len(cfg.Headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithHeaders(cfg.GetHeaders()))
	}

	return grpcExpOpt, nil
}

// httpExporterOptions creates the configuration options for an HTTP-based OTLP metric exporter.
// It configures the exporter with the provided endpoint, URL path, connection security settings, and headers.
func httpExporterOptions(cfg *Config) ([]otlpmetrichttp.Option, error) {
	httpExpOpt := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.Endpoint()),
		otlpmetrichttp.WithURLPath(cfg.HTTPPath),
	}

	if cfg.Insecure {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithInsecure())
	} else {
		tlsCfg, err := common.GetTLSCredentialsForHTTPExporter(
			cfg.CaFile, cfg.ClientAuth, cfg.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithTLSClientConfig(tlsCfg))
	}

	if len(cfg.Headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithHeaders(cfg.GetHeaders()))
	}

	return httpExpOpt, nil
}
