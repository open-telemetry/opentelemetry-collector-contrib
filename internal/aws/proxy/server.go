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

	awsCfg, err := getAWSConfig(cfg, logger)
	if err != nil {
		return nil, err
	}

	// Determine AWS service endpoint
	var serviceEndpoint string
	if cfg.AWSEndpoint != "" {
		serviceEndpoint = cfg.AWSEndpoint
	} else {
		// Constructing the endpoint URL format for the region
		serviceEndpoint = fmt.Sprintf("https://%s.%s.amazonaws.com", cfg.ServiceName, awsCfg.Region)
	}

	// Parse url from endpoint
	awsURL, err := url.Parse(serviceEndpoint)
	if err != nil {
		return nil, fmt.Errorf("unable to parse AWS service endpoint: %w", err)
	}

	// Create credentials provider from the AWS config
	credsProvider := awsCfg.Credentials

	// Create a signer that will sign requests
	signer := v4.NewSigner(func(options *v4.SignerOptions) {
		options.Logger = awsCfg.Logger
		options.LogSigning = awsCfg.ClientLogMode.IsSigning()
	})

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

			// Consume body and convert to io.ReadSeeker for signer to consume
			body, err := consume(req.Body)
			if err != nil {
				logger.Error("Unable to consume request body", zap.Error(err))
				// Forward unsigned request
				return
			}

			// Sign the request using AWS SDK v2
			ctx := context.Background()
			creds, err := credsProvider.Retrieve(ctx)
			if err != nil {
				logger.Error("Unable to retrieve credentials", zap.Error(err))
				return
			}

			// Get payload hash
			var payloadHash string
			if body == nil {
				payloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // SHA256 hash of empty string
			} else {
				bodyBytes, _ := io.ReadAll(body)
				hash := sha256.Sum256(bodyBytes)
				payloadHash = hex.EncodeToString(hash[:])
				// Reset the reader
				body = bytes.NewReader(bodyBytes)
				// Reset the request body
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			// Create signing request
			signReq, err := http.NewRequest(req.Method, req.URL.String(), io.NopCloser(body))
			if err != nil {
				logger.Error("Failed to create signing request", zap.Error(err))
				return
			}

			// Copy headers from original request to signing request
			for k, vv := range req.Header {
				for _, v := range vv {
					signReq.Header.Add(k, v)
				}
			}

			// Sign the request
			err = signer.SignHTTP(ctx, creds, signReq, payloadHash, cfg.ServiceName, awsCfg.Region, time.Now())
			if err != nil {
				logger.Error("Unable to sign request", zap.Error(err))
				return
			}

			// Copy signed headers back to original request
			for k, vv := range signReq.Header {
				req.Header.Del(k)
				for _, v := range vv {
					req.Header.Add(k, v)
				}
			}
		},
	}

	return &http.Server{
		Addr:              cfg.Endpoint,
		Handler:           handler,
		ReadHeaderTimeout: 20 * time.Second,
	}, nil
}

// consume readsAll() the body and creates a new io.ReadSeeker from the content. v4.Signer
// requires an io.ReadSeeker to be able to sign requests. May return a nil io.ReadSeeker.
func consume(body io.ReadCloser) (io.ReadSeeker, error) {
	var buf []byte

	// Return nil ReadSeeker if body is nil
	if body == nil {
		return nil, nil
	}

	// Consume body
	buf, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(buf), nil
}