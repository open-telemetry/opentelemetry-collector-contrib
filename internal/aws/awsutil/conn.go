// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package awsutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

// newHTTPClient returns new HTTP client instance with provided configuration.
func newHTTPClient(logger *zap.Logger, maxIdle int, requestTimeout int, noVerify bool,
	proxyAddress string,
) (*http.Client, error) {
	logger.Debug("Using proxy address: ",
		zap.String("proxyAddr", proxyAddress),
	)
	tls := &tls.Config{
		InsecureSkipVerify: noVerify,
	}

	finalProxyAddress := getProxyAddress(proxyAddress)
	proxyURL, err := getProxyURL(finalProxyAddress)
	if err != nil {
		logger.Error("unable to obtain proxy URL", zap.Error(err))
		return nil, err
	}
	transport := &http.Transport{
		MaxIdleConnsPerHost: maxIdle,
		TLSClientConfig:     tls,
		Proxy:               http.ProxyURL(proxyURL),
	}

	// is not enabled by default as we configure TLSClientConfig for supporting SSL to data plane.
	// http2.ConfigureTransport will setup transport layer to use HTTP2
	if err = http2.ConfigureTransport(transport); err != nil {
		logger.Error("unable to configure http2 transport", zap.Error(err))
		return nil, err
	}

	http := &http.Client{
		Transport: transport,
		Timeout:   time.Second * time.Duration(requestTimeout),
	}
	return http, err
}

func getProxyAddress(proxyAddress string) string {
	var finalProxyAddress string
	switch {
	case proxyAddress != "":
		finalProxyAddress = proxyAddress

	case proxyAddress == "" && os.Getenv("HTTPS_PROXY") != "":
		finalProxyAddress = os.Getenv("HTTPS_PROXY")
	default:
		finalProxyAddress = ""
	}
	return finalProxyAddress
}

func getProxyURL(finalProxyAddress string) (*url.URL, error) {
	var proxyURL *url.URL
	var err error
	if finalProxyAddress != "" {
		proxyURL, err = url.Parse(finalProxyAddress)
	} else {
		proxyURL = nil
		err = nil
	}
	return proxyURL, err
}

// GetAWSConfig returns AWS config instance.
func GetAWSConfig(ctx context.Context, logger *zap.Logger, settings *AWSSessionSettings) (aws.Config, error) {
	http, err := newHTTPClient(logger, settings.NumberOfWorkers, settings.RequestTimeoutSeconds, settings.NoVerifySSL, settings.ProxyAddress)
	if err != nil {
		logger.Error("unable to obtain proxy URL", zap.Error(err))
		return aws.Config{}, err
	}

	var options []func(*config.LoadOptions) error
	options = append(options, config.WithEC2IMDSRegion())

	if settings.Region != "" {
		options = append(options, config.WithRegion(settings.Region))
		logger.Debug("Fetch region from commandline/config file", zap.String("region", settings.Region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return aws.Config{}, err
	}

	if cfg.Region == "" {
		logger.Error("cannot fetch region variable from config file, environment variables and ec2 metadata")
		return aws.Config{}, errors.New("cannot fetch region variable from config file, environment variables and ec2 metadata")
	}

	cfg.HTTPClient = http
	cfg.RetryMaxAttempts = settings.MaxRetries
	if settings.Endpoint != "" {
		cfg.BaseEndpoint = aws.String(settings.Endpoint)
	}

	return cfg, nil
}
