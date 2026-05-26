// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package proxy provides an http server to act as a signing proxy for SDKs calling AWS X-Ray APIs
package proxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

// Server represents HTTP server.
type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

var _ Server = (*server)(nil)

type server struct {
	server       *http.Server
	serverConfig *confighttp.ServerConfig
}

func (s *server) ListenAndServe() error {
	listener, err := s.serverConfig.ToListener(context.Background())
	if err != nil {
		return err
	}

	return s.server.Serve(listener)
}

func (s *server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// NewServer returns a local TCP server that proxies requests to AWS
// backend using the given credentials.
func NewServer(cfg *Config, host component.Host, settings component.TelemetrySettings) (Server, error) {
	_, err := net.ResolveTCPAddr("tcp", cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	if cfg.ProxyAddress != "" {
		settings.Logger.Debug("Using remote proxy", zap.String("address", cfg.ProxyAddress))
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "xray"
	}

	ctx := context.Background()
	awsCfg, err := getAWSConfigSession(ctx, cfg, settings.Logger)
	if err != nil {
		return nil, err
	}

	var awsEndPoint string
	if cfg.AWSEndpoint != "" {
		awsEndPoint = cfg.AWSEndpoint
	} else {
		awsEndPoint, err = getServiceEndpoint(awsCfg.Region, cfg.ServiceName)
		if err != nil {
			return nil, err
		}
	}

	// Parse url from endpoint
	awsURL, err := url.Parse(awsEndPoint)
	if err != nil {
		return nil, fmt.Errorf("unable to parse AWS service endpoint: %w", err)
	}

	signer := v4.NewSigner()
	credentials := awsCfg.Credentials

	transport, err := proxyServerTransport(cfg)
	if err != nil {
		return nil, err
	}

	region := awsCfg.Region
	serviceName := cfg.ServiceName

	// Reverse proxy handler
	handler := &httputil.ReverseProxy{
		Transport: transport,

		Rewrite: func(r *httputil.ProxyRequest) {
			if r.In.URL != nil {
				settings.Logger.Debug("Received request on X-Ray receiver TCP proxy server", zap.String("URL", sanitize.URL(r.In.URL)))
			}

			// Set outbound request URL to the AWS endpoint
			r.Out.URL.Scheme = awsURL.Scheme
			r.Out.URL.Host = awsURL.Host
			r.Out.Host = awsURL.Host

			// Consume body and calculate payload hash for signing
			body, payloadHash, bodyErr := consumeBody(r.In.Body)
			if bodyErr != nil {
				settings.Logger.Error("Unable to consume request body", zap.Error(bodyErr))
				return
			}

			// Restore body for the request
			if body != nil {
				r.Out.Body = io.NopCloser(bytes.NewReader(body))
			}

			// Retrieve credentials for signing
			creds, credsErr := credentials.Retrieve(r.Out.Context())
			if credsErr != nil {
				settings.Logger.Error("Unable to retrieve credentials", zap.Error(credsErr))
				return
			}

			// Sign request using v4 signer
			signErr := signer.SignHTTP(r.Out.Context(), creds, r.Out, payloadHash, serviceName, region, time.Now())
			if signErr != nil {
				settings.Logger.Error("Unable to sign request", zap.Error(signErr))
			}
		},
	}

	serverConfig := confighttp.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  cfg.Endpoint,
			Transport: "tcp",
		},
		ReadHeaderTimeout: 20 * time.Second,
	}

	httpServer, err := serverConfig.ToServer(
		ctx,
		host.GetExtensions(),
		settings,
		handler,
	)
	if err != nil {
		return nil, err
	}

	return &server{
		server:       httpServer,
		serverConfig: &serverConfig,
	}, nil
}

// consumeBody reads the body, calculates SHA-256 hash, and returns the body bytes and hash.
// v4.Signer requires a payload hash for signing.
func consumeBody(body io.ReadCloser) ([]byte, string, error) {
	// Return empty hash if body is nil
	if body == nil {
		// SHA-256 of empty string
		return nil, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil
	}

	// Consume body
	buf, err := io.ReadAll(body)
	if err != nil {
		return nil, "", err
	}

	// Calculate SHA-256 hash
	h := sha256.Sum256(buf)
	payloadHash := hex.EncodeToString(h[:])

	return buf, payloadHash, nil
}
