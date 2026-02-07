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
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
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

// newAWSConfig creates an AWS config using awsutil.GetAWSConfig for basic configuration,
// then applies proxy-specific STS regional endpoint fallback logic for role assumption.
var newAWSConfig = func(ctx context.Context, roleArn, region string, log *zap.Logger) (aws.Config, error) {
	// Use awsutil for basic AWS config loading
	settings := &awsutil.AWSSessionSettings{
		Region:     region,
		MaxRetries: 2,
	}

	cfg, err := awsutil.GetAWSConfig(ctx, log, settings)
	if err != nil {
		return aws.Config{}, err
	}

	if roleArn == "" {
		return cfg, nil
	}

	// Apply proxy-specific STS credentials with regional endpoint fallback
	stsCalls := &stsCallsV2{log: log, getSTSCredsFromRegionEndpoint: getSTSCredsFromRegionEndpointV2}
	creds, err := stsCalls.getCreds(ctx, cfg, region, roleArn)
	if err != nil {
		return aws.Config{}, err
	}
	cfg.Credentials = creds

	return cfg, nil
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

type stsCallsV2 struct {
	log                           *zap.Logger
	getSTSCredsFromRegionEndpoint func(ctx context.Context, log *zap.Logger, cfg aws.Config, region, roleArn string) aws.CredentialsProvider
}

// getCreds gets STS credentials first from the regional endpoint, then from the primary
// region in the respective AWS partition if the regional endpoint is disabled.
func (s *stsCallsV2) getCreds(ctx context.Context, cfg aws.Config, region, roleArn string) (aws.CredentialsProvider, error) {
	stsCred := s.getSTSCredsFromRegionEndpoint(ctx, s.log, cfg, region, roleArn)

	// Make explicit call to fetch credentials to validate they work
	_, err := stsCred.Retrieve(ctx)
	if err != nil {
		var regionDisabledErr *types.RegionDisabledException
		if errors.As(err, &regionDisabledErr) {
			s.log.Warn("STS regional endpoint disabled. Credentials for provided RoleARN will be fetched from STS primary region endpoint instead",
				zap.String("region", region), zap.Error(err))
			return s.getSTSCredsFromPrimaryRegionEndpoint(ctx, cfg, roleArn, region)
		}
		return nil, fmt.Errorf("unable to handle AWS error: %w", err)
	}
	return stsCred, nil
}

// getSTSCredsFromRegionEndpointV2 fetches STS credentials for the provided roleARN
// from the regional endpoint. The STS v2 client's built-in EndpointResolverV2 resolves
// the correct regional endpoint for all AWS partitions (including iso, iso-b, iso-e,
// iso-f, eusc) via its internal awsrulesfn.GetPartition(). We only set Region and let
// the SDK handle endpoint resolution — setting BaseEndpoint would bypass the
// partition-aware logic.
var getSTSCredsFromRegionEndpointV2 = func(_ context.Context, log *zap.Logger, cfg aws.Config, region, roleArn string) aws.CredentialsProvider {
	stsClient := sts.NewFromConfig(cfg, func(o *sts.Options) {
		o.Region = region
	})

	log.Info("STS endpoint resolved by SDK for region", zap.String("region", region))

	return aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(stsClient, roleArn))
}

// getSTSCredsFromPrimaryRegionEndpoint fetches STS credentials for the provided roleARN
// from the primary region endpoint in the respective partition.
func (s *stsCallsV2) getSTSCredsFromPrimaryRegionEndpoint(ctx context.Context, cfg aws.Config, roleArn, region string) (aws.CredentialsProvider, error) {
	primaryRegion, err := getPrimaryRegion(region)
	if err != nil {
		return nil, err
	}
	return s.getSTSCredsFromRegionEndpoint(ctx, s.log, cfg, primaryRegion, roleArn), nil
}

// getPrimaryRegion returns the primary (implicit global) region for the AWS partition
// containing the given region. Primary regions serve as a fallback destination when a
// regional STS endpoint returns RegionDisabledException.
//
// The region-to-partition mapping uses prefix matching that corresponds to the
// regionRegex patterns from the SDK's partitions.json.
func getPrimaryRegion(region string) (string, error) {
	switch {
	case strings.HasPrefix(region, "cn-"):
		return "cn-north-1", nil
	case strings.HasPrefix(region, "us-gov-"):
		return "us-gov-west-1", nil
	case strings.HasPrefix(region, "us-iso-"):
		return "us-iso-east-1", nil
	case strings.HasPrefix(region, "us-isob-"):
		return "us-isob-east-1", nil
	case strings.HasPrefix(region, "eu-isoe-"):
		return "eu-isoe-west-1", nil
	case strings.HasPrefix(region, "us-isof-"):
		return "us-isof-south-1", nil
	case strings.HasPrefix(region, "eusc-"):
		return "eusc-de-east-1", nil
	default:
		// Standard AWS partition (us, eu, ap, sa, ca, me, af, il, mx regions)
		return "us-east-1", nil
	}
}

// isValidRegion checks that the region is a non-empty string containing only
// lowercase ASCII letters, digits, and hyphens (valid DNS label characters).
func isValidRegion(region string) bool {
	if region == "" {
		return false
	}
	for _, c := range region {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
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
