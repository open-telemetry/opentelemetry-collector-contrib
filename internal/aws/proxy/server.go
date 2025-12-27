// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package proxy provides an http server to act as a signing proxy for SDKs calling AWS X-Ray APIs
package proxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

const (
	connHeader = "Connection"
)

// Server represents HTTP server.
type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// NewServer returns a local TCP server that proxies requests to AWS
// backend using the given credentials.
func NewServer(cfg *Config, logger *zap.Logger) (Server, error) {
	_, err := net.ResolveTCPAddr("tcp", cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	if cfg.ProxyAddress != "" {
		logger.Debug("Using remote proxy", zap.String("address", cfg.ProxyAddress))
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "xray"
	}

	ctx := context.Background()
	awsCfg, err := getAWSConfigSession(ctx, cfg, logger)
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

		// Handler for modifying and forwarding requests
		Director: func(req *http.Request) {
			if req != nil && req.URL != nil {
				logger.Debug("Received request on X-Ray receiver TCP proxy server", zap.String("URL", sanitize.URL(req.URL)))
			}

			// Remove connection header before signing request, otherwise the
			// reverse-proxy will remove the header before forwarding to X-Ray
			// resulting in a signed header being missing from the request.
			req.Header.Del(connHeader)

			// Set req url to xray endpoint
			req.URL.Scheme = awsURL.Scheme
			req.URL.Host = awsURL.Host
			req.Host = awsURL.Host

			// Consume body and calculate payload hash for signing
			body, payloadHash, err := consumeBody(req.Body)
			if err != nil {
				logger.Error("Unable to consume request body", zap.Error(err))
				// Forward unsigned request
				return
			}

			// Restore body for the request
			if body != nil {
				req.Body = io.NopCloser(bytes.NewReader(body))
			}

			// Retrieve credentials for signing
			creds, err := credentials.Retrieve(req.Context())
			if err != nil {
				logger.Error("Unable to retrieve credentials", zap.Error(err))
				return
			}

			// Sign request using v4 signer
			err = signer.SignHTTP(req.Context(), creds, req, payloadHash, serviceName, region, time.Now())
			if err != nil {
				logger.Error("Unable to sign request", zap.Error(err))
			}
		},
	}

	return &http.Server{
		Addr:              cfg.Endpoint,
		Handler:           handler,
		ReadHeaderTimeout: 20 * time.Second,
	}, nil
}

// getServiceEndpointFromConfig returns AWS service endpoint.
// If cfg has BaseEndpoint set, it returns that, otherwise it generates from region.
func getServiceEndpointFromConfig(cfg aws.Config, serviceName string) (string, error) {
	if cfg.BaseEndpoint != nil && *cfg.BaseEndpoint != "" {
		return *cfg.BaseEndpoint, nil
	}
	if cfg.Region == "" {
		return "", errors.New("unable to generate endpoint from region with nil value")
	}
	return getServiceEndpoint(cfg.Region, serviceName)
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
