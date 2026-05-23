// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package awsutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

// newHTTPClient returns an aws.HTTPClient backed by an
// *awshttp.BuildableClient. The concrete type matters: the SDK's
// resolveHTTPClient only appends AWS_CA_BUNDLE-derived root CAs when the
// client is a *BuildableClient, so a plain *http.Client would silently
// bypass that handling.
//
// certificateFilePath, when non-empty, is parsed into an empty x509 pool
// (system CAs are intentionally not included; operators who need both
// must combine them in the bundle file).
func newHTTPClient(
	logger *zap.Logger,
	maxIdle, requestTimeout int,
	noVerify bool,
	proxyAddress, certificateFilePath string,
) (aws.HTTPClient, error) {
	rootCAs, certPoolErr := loadCertPool(certificateFilePath)
	if certificateFilePath != "" && certPoolErr != nil {
		logger.Warn("could not create root ca from",
			zap.String("file", certificateFilePath), zap.Error(certPoolErr))
	}

	proxyFunc, err := getProxyFunc(proxyAddress)
	if err != nil {
		logger.Error("unable to obtain proxy URL", zap.Error(err))
		return nil, err
	}

	client := awshttp.NewBuildableClient().
		WithTimeout(time.Duration(requestTimeout) * time.Second).
		WithTransportOptions(func(t *http.Transport) {
			t.MaxIdleConnsPerHost = maxIdle
			t.TLSClientConfig = &tls.Config{
				RootCAs:            rootCAs,
				InsecureSkipVerify: noVerify,
			}
			t.Proxy = proxyFunc
			// Best-effort HTTP/2; safe to ignore the error since the
			// transport falls back to HTTP/1.1.
			_ = http2.ConfigureTransport(t)
		})

	return client, nil
}

// getProxyFunc returns the proxy resolver for an *http.Transport. An
// empty proxyAddress falls through to http.ProxyFromEnvironment, which
// honors HTTP_PROXY, HTTPS_PROXY, and NO_PROXY (upper- and lowercase).
func getProxyFunc(proxyAddress string) (func(*http.Request) (*url.URL, error), error) {
	if proxyAddress == "" {
		return http.ProxyFromEnvironment, nil
	}
	proxyURL, err := url.Parse(proxyAddress)
	if err != nil {
		return nil, err
	}
	return http.ProxyURL(proxyURL), nil
}

// loadCertPool parses a PEM bundle from bundleFile into an empty x509
// pool. System CAs are not included.
func loadCertPool(bundleFile string) (*x509.CertPool, error) {
	bundleBytes, err := os.ReadFile(bundleFile)
	if err != nil {
		return nil, err
	}
	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(bundleBytes) {
		return nil, errors.New("unable to append certs")
	}
	return p, nil
}

// ProxyServerTransport returns an *http.Transport for the X-Ray signing
// proxy. DisableCompression is true so the proxy does not gzip a body
// the upstream client has already SigV4-signed (gzip would invalidate
// the signature).
func ProxyServerTransport(logger *zap.Logger, config *AWSSessionSettings) (*http.Transport, error) {
	proxyFunc, err := getProxyFunc(config.ProxyAddress)
	if err != nil {
		logger.Error("unable to obtain proxy URL", zap.Error(err))
		return nil, err
	}

	return &http.Transport{
		MaxIdleConns:        config.NumberOfWorkers,
		MaxIdleConnsPerHost: config.NumberOfWorkers,
		IdleConnTimeout:     time.Duration(config.RequestTimeoutSeconds) * time.Second,
		Proxy:               proxyFunc,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.NoVerifySSL,
		},
		DisableCompression: true,
	}, nil
}
