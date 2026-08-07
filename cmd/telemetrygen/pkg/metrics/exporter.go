// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/config"
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
		credentials, err := config.GetTLSCredentialsForGRPCExporter(
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

	grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithTimeout(cfg.Timeout))

	return grpcExpOpt, nil
}

// httpExporterOptions creates the configuration options for an HTTP-based OTLP metric exporter.
// It configures the exporter with the provided endpoint, URL path, connection security settings, and headers.
func httpExporterOptions(cfg *Config) ([]otlpmetrichttp.Option, error) {
	endpoint := cfg.Endpoint()
	httpExpOpt := []otlpmetrichttp.Option{otlpmetrichttp.WithURLPath(cfg.HTTPPath)}
	if config.EndpointHasScheme(endpoint) {
		httpExpOpt = append(httpExpOpt,
			otlpmetrichttp.WithEndpointURL(endpoint),
			otlpmetrichttp.WithURLPath(config.HTTPURLPath(endpoint, cfg.HTTPPath)),
		)
	} else {
		endpoint, urlPath := config.ResolveHTTPEndpoint(endpoint, cfg.HTTPPath)
		httpExpOpt = append(httpExpOpt,
			otlpmetrichttp.WithEndpoint(endpoint),
			otlpmetrichttp.WithURLPath(urlPath),
		)
	}

	if cfg.Insecure {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithInsecure())
	} else {
		tlsCfg, err := config.GetTLSCredentialsForHTTPExporter(
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

	httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithTimeout(cfg.Timeout))

	return httpExpOpt, nil
}
