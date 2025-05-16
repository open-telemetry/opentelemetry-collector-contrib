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
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

type ConnAttr interface {
	getEC2Region(ctx context.Context, cfg aws.Config) (string, error)
}

// Conn implements connAttr interface.
type Conn struct{}

func (c *Conn) getEC2Region(ctx context.Context, cfg aws.Config) (string, error) {
	// get region from ec2 metadata service
	client := imds.NewFromConfig(cfg)
	output, err := client.GetRegion(ctx, &imds.GetRegionInput{})
	if err != nil {
		return "", err
	}
	return output.Region, nil
}

// AWS STS endpoint constants
const (
	STSEndpointPrefix         = "https://sts."
	STSEndpointSuffix         = ".amazonaws.com"
	STSAwsCnPartitionIDSuffix = ".amazonaws.com.cn" // AWS China partition
	STSAwsIsoSuffix           = ".c2s.ic.gov"       // AWS ISO partition
	STSAwsIsoBSuffix          = ".sc2s.sgov.gov"    // AWS ISO-B partition
	STSAwsIsoESuffix          = ".cloud.adc-e.uk"   // AWS ISO-E partition
	STSAwsIsoFSuffix          = ".csp.hci.ic.gov"   // AWS ISO-F partition
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
	if settings.Region != "" {
		options = append(options, config.WithRegion(settings.Region))
		logger.Debug("Fetch region from commandline/config file", zap.String("region", settings.Region))
	}

	if settings.RoleARN != "" {
		options = append(options, config.WithAssumeRoleCredentialOptions(func(o *stscreds.AssumeRoleOptions) {
			if settings.ExternalID != "" {
				o.ExternalID = aws.String(settings.ExternalID)
			}
		}))
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

func CreateStaticCredentialProvider(accessKey, secretKey, sessionToken string) aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken)
}

func CreateAssumeRoleCredentialProvider(ctx context.Context, cfg aws.Config, roleARN, externalID string) (aws.CredentialsProvider, error) {
	stsClient := sts.NewFromConfig(cfg)

	options := func(o *stscreds.AssumeRoleOptions) {
		if externalID != "" {
			o.ExternalID = aws.String(externalID)
		}
	}
	provider := stscreds.NewAssumeRoleProvider(stsClient, roleARN, options)
	return aws.NewCredentialsCache(provider), nil
}

// ProxyServerTransport configures HTTP transport for TCP Proxy Server.
func ProxyServerTransport(logger *zap.Logger, config *AWSSessionSettings) (*http.Transport, error) {
	tls := &tls.Config{
		InsecureSkipVerify: config.NoVerifySSL,
	}

	proxyAddr := getProxyAddress(config.ProxyAddress)
	proxyURL, err := getProxyURL(proxyAddr)
	if err != nil {
		logger.Error("unable to obtain proxy URL", zap.Error(err))
		return nil, err
	}

	// Connection timeout in seconds
	idleConnTimeout := time.Duration(config.RequestTimeoutSeconds) * time.Second

	transport := &http.Transport{
		MaxIdleConns:        config.NumberOfWorkers,
		MaxIdleConnsPerHost: config.NumberOfWorkers,
		IdleConnTimeout:     idleConnTimeout,
		Proxy:               http.ProxyURL(proxyURL),
		TLSClientConfig:     tls,

		// If not disabled the transport will add a gzip encoding header
		// to requests with no `accept-encoding` header value. The header
		// is added after we sign the request which invalidates the
		// signature.
		DisableCompression: true,
	}

	return transport, nil
}

// getPartition returns AWS Partition for the provided region.
// AWS SDK v2 provides the GetPartition function to the internal module (https://github.com/aws/aws-sdk-go-v2/blob/main/internal/endpoints/awsrulesfn/partitions.go) and cannot be imported directly.
func getPartition(region string) string {
	// partitions package to determine the correct partition
	if region == "" {
		return "aws" // default partition
	}

	// Check for China regions
	if len(region) >= 2 && region[:2] == "cn" {
		return "aws-cn"
	}

	// Check for US Gov regions
	if len(region) >= 3 && region[:3] == "us-" {
		if len(region) >= 7 && region[:7] == "us-gov-" {
			return "aws-us-gov"
		}
	}

	// Check for AWS ISO regions
	if len(region) >= 6 && region[:6] == "us-iso" {
		if len(region) >= 7 && region[:7] == "us-isob" {
			return "aws-iso-b"
		}
		return "aws-iso"
	}

	// Check for AWS ISO-E regions (European Sovereign Cloud)
	if len(region) >= 7 && region[:7] == "eu-isoe" {
		return "aws-iso-e"
	}

	// Check for AWS ISO-F regions
	if len(region) >= 7 && region[:7] == "us-isof" {
		return "aws-iso-f"
	}

	// Default to standard AWS partition
	return "aws"
}

// getSTSRegionalEndpoint constructs the STS endpoint for a specific region
func getSTSRegionalEndpoint(r string) string {
	p := getPartition(r)

	var suffix string
	switch p {
	case "aws", "aws-us-gov":
		suffix = STSEndpointSuffix
	case "aws-cn":
		suffix = STSAwsCnPartitionIDSuffix
	case "aws-iso":
		suffix = STSAwsIsoSuffix
	case "aws-iso-b":
		suffix = STSAwsIsoBSuffix
	case "aws-iso-e":
		suffix = STSAwsIsoESuffix
	case "aws-iso-f":
		suffix = STSAwsIsoFSuffix
	default:
		suffix = STSEndpointSuffix
	}

	return STSEndpointPrefix + r + suffix
}
