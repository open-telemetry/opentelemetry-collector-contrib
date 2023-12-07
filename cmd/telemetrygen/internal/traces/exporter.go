// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// grpcExporterOptions creates the configuration options for a gRPC-based OTLP trace exporter.
// It configures the exporter with the provided endpoint, connection security settings, and headers.
func grpcExporterOptions(cfg *Config) []otlptracegrpc.Option {
	grpcExpOpt := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint()),
		otlptracegrpc.WithDialOption(
			grpc.WithBlock(),
		),
	}

	if cfg.Insecure {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithInsecure())
	} else {
		credentials, err := GetTLSCredentialsForGRPCExporter(cfg)
		if err != nil {
			fmt.Printf("Failed to get TLS crendentials: %w, exiting.\n", err)
			os.Exit(1)
		} else {
			grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithTLSCredentials(credentials))
		}
	}

	if len(cfg.Headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	return grpcExpOpt
}

// httpExporterOptions creates the configuration options for an HTTP-based OTLP trace exporter.
// It configures the exporter with the provided endpoint, URL path, connection security settings, and headers.
func httpExporterOptions(cfg *Config) []otlptracehttp.Option {
	httpExpOpt := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint()),
		otlptracehttp.WithURLPath(cfg.HTTPPath),
	}

	if cfg.Insecure {
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithInsecure())
	} else {
		tlsCfg, err := GetTLSCredentialsForHTTPExporter(cfg)
		if err != nil {
			fmt.Printf("Failed to get TLS configuration: %w, exiting.\n", err)
			os.Exit(1)
		} else {
			httpExpOpt = append(httpExpOpt, otlptracehttp.WithTLSClientConfig(tlsCfg))
		}
	}

	if len(cfg.Headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlptracehttp.WithHeaders(cfg.Headers))
	}

	return httpExpOpt
}

// caPool loads CA certificate from a file and returns a CertPool. 
// The certPool is used to set RootCAs in certificate verification.
func caPool(cfg *Config) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if cfg.CaFile != "" {
		data, err := os.ReadFile(cfg.CaFile)
		if err != nil {
			return nil, err
		}
		if !pool.AppendCertsFromPEM(data) {
			return nil, errors.New("failed to add CA certificate to root CA pool")
		}
	}
	return pool, nil
}

func GetTLSCredentialsForGRPCExporter(cfg *Config) (credentials.TransportCredentials, error) {

	pool, err := caPool(cfg)
	if err != nil {
		return nil, err
	}

	creds := credentials.NewTLS(&tls.Config{
		RootCAs: pool,
	})

	//Configuration for mTLS
	if cfg.ClientAuth.Enabled {
		keypair, err := tls.LoadX509KeyPair(cfg.ClientAuth.ClientCertFile, cfg.ClientAuth.ClientKeyFile)
		if err != nil {
			return nil, err
		}
		creds = credentials.NewTLS(&tls.Config{
			RootCAs:      pool,
			Certificates: []tls.Certificate{keypair},
		})
	}

	return creds, nil
}

func GetTLSCredentialsForHTTPExporter(cfg *Config) (*tls.Config, error) {
	pool, err := caPool(cfg)
	if err != nil {
		return nil, err
	}

	tlsCfg := tls.Config{
		RootCAs: pool,
	}

	//Configuration for mTLS
	if cfg.ClientAuth.Enabled {
		keypair, err := tls.LoadX509KeyPair(cfg.ClientAuth.ClientCertFile, cfg.ClientAuth.ClientKeyFile)
		if err != nil {
			return nil, err
		}
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		tlsCfg.Certificates = []tls.Certificate{keypair}
	}
	return &tlsCfg, nil
}
