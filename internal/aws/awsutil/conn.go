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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

type ConnAttr interface {
	newAWSSession(logger *zap.Logger, roleArn string, region string) (aws.Config, error)
	getEC2Region(c aws.Config) (string, error)
}

// Conn implements connAttr interface.
type Conn struct{}

func (c *Conn) getEC2Region(s aws.Config) (string, error) {
	imdsClient := imds.NewFromConfig(s)
	regionOutput, err := imdsClient.GetRegion(context.TODO(), &imds.GetRegionInput{})
	if err != nil {
		return "", err
	}
	return regionOutput.Region, nil
}

// AWS STS endpoint constants
const (
	STSEndpointPrefix         = "https://sts."
	STSEndpointSuffix         = ".amazonaws.com"
	STSAwsCnPartitionIDSuffix = ".amazonaws.com.cn" // AWS China partition.
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

// GetAWSConfigSession returns AWS config and session instances.
func GetAWSConfig(logger *zap.Logger, cn ConnAttr, cfg *AWSSessionSettings) (*aws.Config, aws.Config, error) {
	var awsRegion string
	var err error
	httpClient, err := newHTTPClient(logger, cfg.NumberOfWorkers, cfg.RequestTimeoutSeconds, cfg.NoVerifySSL, cfg.ProxyAddress)
	if err != nil {
		logger.Error("Unable to obtain proxy URL", zap.Error(err))
		return nil, aws.Config{}, err
	}

	regionEnv := os.Getenv("AWS_REGION")

	switch {
	case cfg.Region == "" && regionEnv != "":
		awsRegion = regionEnv
		logger.Debug("Fetch region from environment variables", zap.String("region", awsRegion))
	case cfg.Region != "":
		awsRegion = cfg.Region
		logger.Debug("Fetch region from command-line/config file", zap.String("region", awsRegion))
	case !cfg.NoVerifySSL:
		awsCfg, err := GetDefaultConfig(logger)
		if err != nil {
			logger.Error("Unable to retrieve default session", zap.Error(err))
		} else {
			awsRegion, err = cn.getEC2Region(awsCfg)
			if err != nil {
				logger.Error("Unable to retrieve the region from the EC2 instance", zap.Error(err))
			} else {
				logger.Debug("Fetch region from EC2 metadata", zap.String("region", awsRegion))
			}
		}
	}

	if awsRegion == "" {
		msg := "Cannot fetch region variable from config file, environment variables, and EC2 metadata."
		logger.Error(msg)
		return nil, aws.Config{}, errors.New("NoAwsRegion")
	}

	awsCfg, err := cn.newAWSSession(logger, cfg.RoleARN, awsRegion)
	if err != nil {
		logger.Error("Failed to create AWS session", zap.Error(err))
		return nil, aws.Config{}, err
	}

	config := &aws.Config{
		Region:           awsRegion,
		RetryMaxAttempts: cfg.MaxRetries,
		HTTPClient:       httpClient,
	}
	return config, awsCfg, nil
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

func (c *Conn) newAWSSession(logger *zap.Logger, roleArn string, region string) (aws.Config, error) {
	var cfg aws.Config
	var err error
	if roleArn == "" {
		cfg, err = GetDefaultConfig(logger)
		if err != nil {
			return aws.Config{}, err
		}
	} else {
		stsCreds, err := getSTSCreds(logger, region, roleArn)
		if err != nil {
			logger.Error("Error in getting STS credentials: ", zap.Error(err))
			return aws.Config{}, err
		}

		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithCredentialsProvider(stsCreds),
		)
		if err != nil {
			logger.Error("Error in creating session object : ", zap.Error(err))
			return aws.Config{}, err
		}
	}
	return cfg, nil
}

// getSTSCreds gets STS credentials from regional endpoint. ErrCodeRegionDisabledException is received if the
// STS regional endpoint is disabled. In this case STS credentials are fetched from STS primary regional endpoint
// in the respective AWS partition.
func getSTSCreds(logger *zap.Logger, region string, roleArn string) (*stscreds.AssumeRoleProvider, error) {
	t, err := GetDefaultConfig(logger)
	if err != nil {
		return nil, err
	}
	stsCred := getSTSCredsFromRegionEndpoint(logger, t, region, roleArn)
	// Make explicit call to fetch credentials.
	_, err = stsCred.Retrieve(context.TODO())
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			err = nil

			if apiErr.ErrorCode() == "RegionDisabledException" {
				logger.Error("Region ", zap.String("region", region), zap.Error(apiErr))
				stsCred = getSTSCredsFromPrimaryRegionEndpoint(logger, t, roleArn, region)
			}
		}
	}
	return stsCred, err
}

// getSTSCredsFromRegionEndpoint fetches STS credentials for provided roleARN from regional endpoint.
// AWS STS recommends that you provide both the Region and endpoint when you make calls to a Regional endpoint.
// Reference: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html#id_credentials_temp_enable-regions_writing_code
func getSTSCredsFromRegionEndpoint(logger *zap.Logger, conf aws.Config, region string, roleArn string) *stscreds.AssumeRoleProvider {
	regionalEndpoint := getSTSRegionalEndpoint(region)
	// if regionalEndpoint is "", the STS endpoint is Global endpoint for classic regions except ap-east-1 - (HKG)
	// for other opt-in regions, region value will create STS regional endpoint.
	// This will be only in the case, if provided region is not present in aws_regions.go
	st := sts.NewFromConfig(conf, func(o *sts.Options) {
		o.Region = region
		if regionalEndpoint != "" {
			o.BaseEndpoint = &regionalEndpoint
		}
	})
	logger.Info("STS Endpoint", zap.String("endpoint", regionalEndpoint))
	return stscreds.NewAssumeRoleProvider(st, roleArn)
}

// TODO: Refactor this function once the Solution is found to provides a way to get the partition ID from the region.
// The partition ID is used to identify the AWS partition is a temporary solution to get the partition ID from the region.
func getSTSCredsFromPrimaryRegionEndpoint(logger *zap.Logger, t aws.Config, roleArn string, region string) *stscreds.AssumeRoleProvider {
	logger.Info("Credentials for provided RoleARN being fetched from STS primary region endpoint.")
	partitionID := getPartition(region)
	var primaryRegion string
	switch partitionID {
	case "aws":
		primaryRegion = "us-east-1"
	case "aws-cn":
		primaryRegion = "cn-north-1"
	case "aws-us-gov":
		primaryRegion = "us-gov-west-1"
	default:
		logger.Error("Unsupported partition ID")
		return nil
	}
	return getSTSCredsFromRegionEndpoint(logger, t, primaryRegion, roleArn)
}

// getSTSRegionalEndpoint returns the regional endpoint for the provided region.
// This is a temporary solution to get the regional endpoint from the region.
func getSTSRegionalEndpoint(region string) string {
	partition := getPartition(region)
	switch partition {
	case "aws", "aws-us-gov":
		return STSEndpointPrefix + region + STSEndpointSuffix
	case "aws-cn":
		return STSEndpointPrefix + region + STSAwsCnPartitionIDSuffix
	default:
		return ""
	}
}

func GetDefaultConfig(logger *zap.Logger) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		logger.Error("Error in creating session object ", zap.Error(err))
		return aws.Config{}, err
	}
	return cfg, nil
}

// Currently, `endpoints` from AWS SDK Go v2 docs does not provide a way to get the partition ID from the region.
// This function is a temporary solution to get the partition ID from the region.
func getPartition(region string) string {
	switch {
	case strings.HasPrefix(region, "cn-"):
		return "aws-cn" // AWS China Partition
	case strings.HasPrefix(region, "us-gov-"):
		return "aws-us-gov" // AWS GovCloud Partition
	case strings.HasPrefix(region, "us"):
		return "aws" // AWS Standard Partition
	case strings.HasPrefix(region, "aws"):
		return "aws" // AWS Partition
	default:
		return ""
	}
}
