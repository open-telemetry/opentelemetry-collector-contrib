// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/config"
)

type profileExporter interface {
	Export(ctx context.Context, td pprofile.Profiles) error
	Shutdown(ctx context.Context) error
}

type grpcProfileExporter struct {
	client pprofileotlp.GRPCClient
	conn   *grpc.ClientConn
}

func newGRPCExporter(cfg *Config) (*grpcProfileExporter, error) {
	var opts []grpc.DialOption
	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		credentials, err := config.GetTLSCredentialsForGRPCExporter(
			cfg.CaFile, cfg.ClientAuth, cfg.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials))
	}

	if len(cfg.Headers) > 0 {
		md := metadata.New(cfg.GetHeaders())
		opts = append(opts, grpc.WithUnaryInterceptor(
			func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
				return invoker(metadata.NewOutgoingContext(ctx, md), method, req, reply, cc, callOpts...)
			},
		))
	}

	conn, err := grpc.NewClient(cfg.Endpoint(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC endpoint %s: %w", cfg.Endpoint(), err)
	}

	return &grpcProfileExporter{
		client: pprofileotlp.NewGRPCClient(conn),
		conn:   conn,
	}, nil
}

func (e *grpcProfileExporter) Export(ctx context.Context, td pprofile.Profiles) error {
	req := pprofileotlp.NewExportRequestFromProfiles(td)
	_, err := e.client.Export(ctx, req)
	return err
}

func (e *grpcProfileExporter) Shutdown(_ context.Context) error {
	return e.conn.Close()
}

type httpProfileExporter struct {
	client   *http.Client
	endpoint string
	headers  map[string]string
}

func newHTTPExporter(cfg *Config) (*httpProfileExporter, error) {
	scheme := "https"
	if cfg.Insecure {
		scheme = "http"
	}
	endpoint := fmt.Sprintf("%s://%s%s", scheme, cfg.Endpoint(), cfg.HTTPPath)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !cfg.Insecure {
		tlsCfg, err := config.GetTLSCredentialsForHTTPExporter(
			cfg.CaFile, cfg.ClientAuth, cfg.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		transport.TLSClientConfig = tlsCfg
	}

	return &httpProfileExporter{
		client:   &http.Client{Transport: transport},
		endpoint: endpoint,
		headers:  cfg.GetHeaders(),
	}, nil
}

func (e *httpProfileExporter) Export(ctx context.Context, td pprofile.Profiles) error {
	req := pprofileotlp.NewExportRequestFromProfiles(td)
	body, err := req.MarshalProto()
	if err != nil {
		return fmt.Errorf("failed to marshal profiles: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	for k, v := range e.headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP export failed with status %d", resp.StatusCode)
	}
	return nil
}

func (e *httpProfileExporter) Shutdown(_ context.Context) error {
	e.client.CloseIdleConnections()
	return nil
}
