// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package proxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

const (
	idleConnTimeout                = 30 * time.Second
	remoteProxyMaxIdleConnsPerHost = 2

	awsRegionEnvVar                   = "AWS_REGION"
	awsDefaultRegionEnvVar            = "AWS_DEFAULT_REGION"
	ecsContainerMetadataEnabledEnvVar = "ECS_ENABLE_CONTAINER_METADATA"
	ecsMetadataFileEnvVar             = "ECS_CONTAINER_METADATA_FILE"
)

var getEC2Region = func(ctx context.Context, cfg aws.Config) (string, error) {
	client := imds.NewFromConfig(cfg)
	output, err := client.GetRegion(ctx, &imds.GetRegionInput{})
	if err != nil {
		return "", err
	}
	return output.Region, nil
}

// newAWSConfig creates an AWS config using awsutil.GetAWSConfig, which handles
// region setting, retry configuration, and STS AssumeRole credentials if a role
// ARN is provided. The SDK's built-in EndpointResolverV2 handles partition-aware
// endpoint resolution for all AWS partitions.
var newAWSConfig = func(ctx context.Context, roleArn, region string, log *zap.Logger) (aws.Config, error) {
	settings := &awsutil.AWSSessionSettings{
		Region:     region,
		RoleARN:    roleArn,
		MaxRetries: 2,
	}
	return awsutil.GetAWSConfig(ctx, log, settings)
}

func getAWSConfigSession(ctx context.Context, c *Config, logger *zap.Logger) (aws.Config, error) {
	region, err := resolveRegion(ctx, c, logger)
	if err != nil {
		return aws.Config{}, err
	}

	cfg, err := newAWSConfig(ctx, c.RoleARN, region, logger)
	if err != nil {
		return aws.Config{}, err
	}

	if c.AWSEndpoint != "" {
		cfg.BaseEndpoint = aws.String(c.AWSEndpoint)
	}

	return cfg, nil
}

// resolveRegion determines the AWS region using the following priority:
//  1. Config file (c.Region)
//  2. Environment variables (AWS_DEFAULT_REGION, AWS_REGION)
//  3. ECS container metadata file (proxy-specific, not in awsutil)
//  4. EC2 instance metadata service (IMDS)
func resolveRegion(ctx context.Context, c *Config, logger *zap.Logger) (string, error) {
	// 1. Config file takes highest priority
	if c.Region != "" {
		logger.Debug("Fetched region from config file", zap.String("region", c.Region))
		return c.Region, nil
	}

	// 2. Environment variables
	if region := getRegionFromEnv(); region != "" {
		logger.Debug("Fetched region from environment variables", zap.String("region", region))
		return region, nil
	}

	// 3 & 4. Metadata services (only if not in local mode)
	if c.LocalMode {
		return "", errors.New("region not specified and local mode enabled; cannot fetch from metadata services")
	}

	return getRegionFromMetadata(ctx, logger)
}

// getRegionFromEnv returns the region from environment variables.
// AWS_DEFAULT_REGION takes precedence over AWS_REGION.
func getRegionFromEnv() string {
	if region := os.Getenv(awsDefaultRegionEnvVar); region != "" {
		return region
	}
	return os.Getenv(awsRegionEnvVar)
}

// getRegionFromMetadata attempts to get region from ECS metadata first,
// then falls back to EC2 IMDS.
func getRegionFromMetadata(ctx context.Context, logger *zap.Logger) (string, error) {
	// Try ECS metadata first (proxy-specific feature not in awsutil)
	region, err := getRegionFromECSMetadata()
	if err == nil {
		logger.Debug("Fetched region from ECS metadata file", zap.String("region", region))
		return region, nil
	}
	logger.Debug("Unable to fetch region from ECS metadata", zap.Error(err))

	// Fall back to EC2 IMDS
	region, ec2Err := getRegionFromEC2Metadata(ctx, logger)
	if ec2Err == nil {
		logger.Debug("Fetched region from EC2 metadata", zap.String("region", region))
		return region, nil
	}
	logger.Debug("Unable to fetch region from EC2 metadata", zap.Error(ec2Err))

	// Return combined error
	return "", fmt.Errorf("could not fetch region from config file, environment variables, ecs metadata, or ec2 metadata: ECS: %w; EC2: %w", err, ec2Err)
}

// getRegionFromEC2Metadata fetches region from EC2 Instance Metadata Service.
var getRegionFromEC2Metadata = func(ctx context.Context, logger *zap.Logger) (string, error) {
	tempSettings := &awsutil.AWSSessionSettings{}
	tempCfg, err := awsutil.GetAWSConfig(ctx, logger, tempSettings)
	if err != nil {
		return "", err
	}
	return getEC2Region(ctx, tempCfg)
}

func getRegionFromECSMetadata() (string, error) {
	ecsMetadataEnabled := os.Getenv(ecsContainerMetadataEnabledEnvVar)
	ecsMetadataEnabled = strings.ToLower(ecsMetadataEnabled)
	if ecsMetadataEnabled == "true" {
		metadataFilePath := os.Getenv(ecsMetadataFileEnvVar)
		metadata, err := os.ReadFile(metadataFilePath)
		if err != nil {
			return "", fmt.Errorf("unable to open ECS metadata file, path: %s, error: %w",
				metadataFilePath, err)
		}
		var dat map[string]any
		err = json.Unmarshal(metadata, &dat)
		if err != nil {
			return "", fmt.Errorf("invalid json in read ECS metadata file content, path: %s, error: %w",
				metadataFilePath, err)
		}
		taskArn, err := arn.Parse(dat["TaskARN"].(string))
		if err != nil {
			return "", err
		}

		return taskArn.Region, nil
	}
	return "", errors.New("ECS metadata endpoint is inaccessible")
}

// proxyServerTransport configures HTTP transport for TCP Proxy Server.
func proxyServerTransport(cfg *Config) (*http.Transport, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.TLS.Insecure,
	}

	proxyAddr := awsutil.GetProxyAddress(cfg.ProxyAddress)
	proxyURL, err := awsutil.GetProxyURL(proxyAddr)
	if err != nil {
		return nil, err
	}

	return &http.Transport{
		MaxIdleConnsPerHost: remoteProxyMaxIdleConnsPerHost,
		IdleConnTimeout:     idleConnTimeout,
		Proxy:               http.ProxyURL(proxyURL),
		TLSClientConfig:     tlsCfg,

		// If not disabled the transport will add a gzip encoding header
		// to requests with no `accept-encoding` header value. The header
		// is added after we sign the request which invalidates the
		// signature.
		DisableCompression: true,
	}, nil
}

// isValidRegion checks that the region is a non-empty string containing only
// lowercase ASCII letters, digits, and hyphens (valid DNS label characters).
func isValidRegion(region string) bool {
	if region == "" {
		return false
	}
	for _, c := range region {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// getServiceEndpoint returns the AWS service endpoint URL for a given service and region.
// It leverages the STS EndpointResolverV2 (which internally uses awsrulesfn.GetPartition
// covering all 8 AWS partitions) to resolve the correct DNS suffix, then replaces the
// service name in the resolved URL.
func getServiceEndpoint(region, serviceName string) (string, error) {
	if !isValidRegion(region) {
		return "", fmt.Errorf("invalid region: %s", region)
	}
	resolver := sts.NewDefaultEndpointResolverV2()
	endpoint, err := resolver.ResolveEndpoint(context.Background(), sts.EndpointParameters{
		Region: aws.String(region),
	})
	if err != nil {
		return "", fmt.Errorf("invalid region: %s", region)
	}
	// The STS endpoint URL has the format https://sts.<region>.<dnsSuffix>.
	// Replace the "sts" service prefix with the target service name.
	u := endpoint.URI.String()
	return strings.Replace(u, "://sts.", "://"+serviceName+".", 1), nil
}
