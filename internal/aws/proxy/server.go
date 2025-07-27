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
	connHeader       = "Connection"
	emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
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

	awsCfg, err := getAWSConfig(cfg, logger)
	if err != nil {
		return nil, err
	}

	awsEndPoint, err := getServiceEndpoint(awsCfg, cfg.ServiceName)
	if err != nil {
		return nil, err
	}

	// Parse url from endpoint
	awsURL, err := url.Parse(awsEndPoint)
	if err != nil {
		return nil, fmt.Errorf("unable to parse AWS service endpoint: %w", err)
	}

	signer := v4.NewSigner()

	transport, err := proxyServerTransport(cfg)
	if err != nil {
		return nil, err
	}

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

			payloadHash, err := calculatePayloadHash(req)
			if err != nil {
				logger.Error("unable to calculate payload hash", zap.Error(err))
				return
			}

			creds, err := awsCfg.Credentials.Retrieve(context.Background())
			if err != nil {
				logger.Error("unable to retrieve credentials", zap.Error(err))
				return
			}

			err = signer.SignHTTP(context.Background(), creds, req, payloadHash, cfg.ServiceName, awsCfg.Region, time.Now())
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

// getServiceEndpoint constructs AWS service endpoints using partition-aware logic.
func getServiceEndpoint(awsCfg aws.Config, serviceName string) (string, error) {
	if awsCfg.BaseEndpoint != nil && *awsCfg.BaseEndpoint != "" {
		return *awsCfg.BaseEndpoint, nil
	}

	if awsCfg.Region == "" {
		return "", errors.New("unable to generate endpoint from region with empty value")
	}

	return buildServiceEndpoint(awsCfg.Region, serviceName)
}

// buildServiceEndpoint constructs service endpoints using partition-aware logic.
func buildServiceEndpoint(region, serviceName string) (string, error) {
	partition := getPartition(region)
	switch partition {
	case awsPartition, awsGovCloudPartition:
		return fmt.Sprintf("https://%s.%s.amazonaws.com", serviceName, region), nil
	case awsChinaPartition:
		return fmt.Sprintf("https://%s.%s.amazonaws.com.cn", serviceName, region), nil
	default:
		return fmt.Sprintf("https://%s.%s.amazonaws.com", serviceName, region), nil
	}
}

// calculatePayloadHash calculates the SHA256 hash of the request payload.
// For requests with no body, returns the hash of an empty string.
func calculatePayloadHash(req *http.Request) (string, error) {
	if req.Body == nil {
		return emptyPayloadHash, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read request body: %w", err)
	}

	hash := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(hash[:])

	req.Body = io.NopCloser(bytes.NewReader(body))

	return payloadHash, nil
}
