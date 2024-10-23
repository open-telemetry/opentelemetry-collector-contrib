// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// grpcExporterOptions creates the configuration options for a gRPC-based OTLP log exporter.
// It configures the exporter with the provided endpoint, connection security settings, and headers.
func grpcExporterOptions(cfg *Config) ([]otlploggrpc.Option, error) {
	grpcExpOpt := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.Endpoint()),
	}

	if cfg.Insecure {
		grpcExpOpt = append(grpcExpOpt, otlploggrpc.WithInsecure())
	} else {
		credentials, err := common.GetTLSCredentialsForGRPCExporter(
			cfg.CaFile, cfg.ClientAuth, cfg.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		grpcExpOpt = append(grpcExpOpt, otlploggrpc.WithTLSCredentials(credentials))
	}

	if len(cfg.Headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlploggrpc.WithHeaders(cfg.GetHeaders()))
	}

	return grpcExpOpt, nil
}

// httpExporterOptions creates the configuration options for an HTTP-based OTLP log exporter.
// It configures the exporter with the provided endpoint, URL path, connection security settings, and headers.
func httpExporterOptions(cfg *Config) ([]otlploghttp.Option, error) {
	httpExpOpt := []otlploghttp.Option{
		otlploghttp.WithEndpoint(cfg.Endpoint()),
		otlploghttp.WithURLPath(cfg.HTTPPath),
	}

	if cfg.Insecure {
		httpExpOpt = append(httpExpOpt, otlploghttp.WithInsecure())
	} else {
		tlsCfg, err := common.GetTLSCredentialsForHTTPExporter(
			cfg.CaFile, cfg.ClientAuth, cfg.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		httpExpOpt = append(httpExpOpt, otlploghttp.WithTLSClientConfig(tlsCfg))
	}

	if len(cfg.Headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlploghttp.WithHeaders(cfg.GetHeaders()))
	}

	return httpExpOpt, nil
}
