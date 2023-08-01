// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package proxy provides an http server to act as a signing proxy for SDKs calling AWS X-Ray APIs
package proxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

const (
	service    = "xray"
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

	awsCfg, sess, err := getAWSConfigSession(cfg, logger)
	if err != nil {
		return nil, err
	}

	awsEndPoint, err := getServiceEndpoint(awsCfg)
	if err != nil {
		return nil, err
	}

	// Parse url from endpoint
	awsURL, err := url.Parse(awsEndPoint)
	if err != nil {
		return nil, fmt.Errorf("unable to parse AWS service endpoint: %w", err)
	}

	signer := &v4.Signer{
		Credentials: sess.Config.Credentials,
	}

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

			// Sign request. signer.Sign() also repopulates the request body.
			_, err = signer.Sign(req, body, service, *awsCfg.Region, time.Now())
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

// getServiceEndpoint returns X-Ray service endpoint.
// It is guaranteed that awsCfg config instance is non-nil and the region value is non nil or non empty in awsCfg object.
// Currently the caller takes care of it.
func getServiceEndpoint(awsCfg *aws.Config) (string, error) {
	if isEmpty(awsCfg.Endpoint) {
		if isEmpty(awsCfg.Region) {
			return "", errors.New("unable to generate endpoint from region with nil value")
		}
		resolved, err := endpoints.DefaultResolver().EndpointFor(service, *awsCfg.Region, setResolverConfig())
		return resolved.URL, err
	}
	return *awsCfg.Endpoint, nil
}

func isEmpty(val *string) bool {
	return val == nil || *val == ""
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

func setResolverConfig() func(*endpoints.Options) {
	return func(p *endpoints.Options) {
		p.ResolveUnknownService = true
	}
}
