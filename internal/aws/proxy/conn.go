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
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"go.uber.org/zap"
)

const (
	idleConnTimeout                = 30 * time.Second
	remoteProxyMaxIdleConnsPerHost = 2

	awsRegionEnvVar                   = "AWS_REGION"
	awsDefaultRegionEnvVar            = "AWS_DEFAULT_REGION"
	ecsContainerMetadataEnabledEnvVar = "ECS_ENABLE_CONTAINER_METADATA"
	ecsMetadataFileEnvVar             = "ECS_CONTAINER_METADATA_FILE"

	httpsProxyEnvVar = "HTTPS_PROXY"

	awsPartition         = "aws"
	awsChinaPartition    = "aws-cn"
	awsGovCloudPartition = "aws-us-gov"

	awsPrimaryRegion         = "us-east-1"
	awsChinaPrimaryRegion    = "cn-north-1"
	awsGovCloudPrimaryRegion = "us-gov-west-1"
)

var newAWSConfig = func(roleArn, region string, log *zap.Logger) (aws.Config, error) {
	ctx := context.Background()
	sts := &stsCalls{log: log, getSTSCredsFromRegionEndpoint: getSTSCredsFromRegionEndpoint}

	if roleArn == "" {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			return aws.Config{}, err
		}
		return cfg, nil
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return aws.Config{}, err
	}

	stsCreds, err := sts.getCreds(region, roleArn, cfg)
	if err != nil {
		return aws.Config{}, err
	}

	cfg.Credentials = stsCreds
	return cfg, nil
}

var getEC2Region = func(cfg aws.Config) (string, error) {
	client := imds.NewFromConfig(cfg)
	result, err := client.GetRegion(context.Background(), &imds.GetRegionInput{})
	if err != nil {
		return "", err
	}
	return result.Region, nil
}

func getAWSConfig(c *Config, logger *zap.Logger) (aws.Config, error) {
	var (
		awsRegion string
		err       error
	)
	regionEnv := os.Getenv(awsDefaultRegionEnvVar)
	if regionEnv == "" {
		regionEnv = os.Getenv(awsRegionEnvVar)
	}

	switch {
	case c.Region == "" && regionEnv != "":
		awsRegion = regionEnv
		logger.Debug("Fetched region from environment variables", zap.String("region", awsRegion))
	case c.Region != "":
		awsRegion = c.Region
		logger.Debug("Fetched region from config file", zap.String("region", awsRegion))
	case !c.LocalMode:
		awsRegion, err = getRegionFromECSMetadata()
		if err != nil {
			logger.Debug("Unable to fetch region from ECS metadata", zap.Error(err))
			var cfg aws.Config
			cfg, err = config.LoadDefaultConfig(context.Background())
			if err == nil {
				awsRegion, err = getEC2Region(cfg)
				if err != nil {
					logger.Debug("Unable to fetch region from EC2 metadata", zap.Error(err))
				} else {
					logger.Debug("Fetched region from EC2 metadata", zap.String("region", awsRegion))
				}
			}
		} else {
			logger.Debug("Fetched region from ECS metadata file", zap.String("region", awsRegion))
		}
	}

	if err != nil {
		return aws.Config{}, fmt.Errorf("could not fetch region from config file, environment variables, ecs metadata, or ec2 metadata: %w", err)
	}

	cfg, err := newAWSConfig(c.RoleARN, awsRegion, logger)
	if err != nil {
		return aws.Config{}, err
	}

	if c.AWSEndpoint != "" {
		cfg.BaseEndpoint = &c.AWSEndpoint
	}

	return cfg, nil
}

func getProxyAddress(proxyAddress string) string {
	if proxyAddress != "" {
		return proxyAddress
	}
	if os.Getenv(httpsProxyEnvVar) != "" {
		return os.Getenv(httpsProxyEnvVar)
	}
	return ""
}

func getProxyURL(proxyAddress string) (*url.URL, error) {
	var proxyURL *url.URL
	var err error
	if proxyAddress != "" {
		proxyURL, err = url.Parse(proxyAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
		}
		return proxyURL, nil
	}
	return nil, nil
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
func proxyServerTransport(config *Config) (*http.Transport, error) {
	tls := &tls.Config{
		InsecureSkipVerify: config.TLS.Insecure,
	}

	proxyAddr := getProxyAddress(config.ProxyAddress)
	proxyURL, err := getProxyURL(proxyAddr)
	if err != nil {
		return nil, err
	}

	return &http.Transport{
		MaxIdleConnsPerHost: remoteProxyMaxIdleConnsPerHost,
		IdleConnTimeout:     idleConnTimeout,
		Proxy:               http.ProxyURL(proxyURL),
		TLSClientConfig:     tls,

		// If not disabled the transport will add a gzip encoding header
		// to requests with no `accept-encoding` header value. The header
		// is added after we sign the request which invalidates the
		// signature.
		DisableCompression: true,
	}, nil
}

type stsCalls struct {
	log                           *zap.Logger
	getSTSCredsFromRegionEndpoint func(log *zap.Logger, cfg aws.Config, region, roleArn string) aws.CredentialsProvider
}

// getCreds gets STS credentials first from the regional endpoint, then from the primary
// region in the respective AWS partition if the regional endpoint is disabled.
func (s *stsCalls) getCreds(region, roleArn string, baseCfg aws.Config) (aws.CredentialsProvider, error) {
	stsCred := s.getSTSCredsFromRegionEndpoint(s.log, baseCfg, region, roleArn)

	// Test credentials retrieval to detect RegionDisabledException early.
	_, err := stsCred.Retrieve(context.Background())
	var regionDisabledErr *types.RegionDisabledException
	if err != nil && errors.As(err, &regionDisabledErr) {
		s.log.Warn("STS regional endpoint disabled. Credentials for provided RoleARN will be fetched from STS primary region endpoint instead",
			zap.String("region", region), zap.Error(regionDisabledErr))
		stsCred, err = s.getSTSCredsFromPrimaryRegionEndpoint(baseCfg, roleArn, region)
	}

	if err != nil {
		return nil, fmt.Errorf("unable to retrieve STS credentials: %w", err)
	}

	return stsCred, err
}

// getSTSCredsFromRegionEndpoint fetches STS credentials for provided roleARN from regional endpoint.
// AWS STS recommends that you provide both the Region and endpoint when you make calls to a Regional endpoint.
// Reference: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html#id_credentials_temp_enable-regions_writing_code
var getSTSCredsFromRegionEndpoint = func(log *zap.Logger, cfg aws.Config, region, roleArn string) aws.CredentialsProvider {
	regionalEndpoint := getSTSRegionalEndpoint(region)
	// if regionalEndpoint is "", the STS endpoint is Global endpoint for classic regions except ap-east-1 - (HKG)
	// for other opt-in regions, region value will create STS regional endpoint.
	// This will only be the case if the provided region is not present in aws_regions.go

	cfgCopy := cfg.Copy()
	cfgCopy.Region = region
	if regionalEndpoint != "" {
		cfgCopy.BaseEndpoint = &regionalEndpoint
	}

	stsClient := sts.NewFromConfig(cfgCopy)
	log.Info("STS endpoint to use", zap.String("endpoint", regionalEndpoint))

	return stscreds.NewAssumeRoleProvider(stsClient, roleArn)
}

// getSTSCredsFromPrimaryRegionEndpoint fetches STS credentials for provided roleARN from primary region endpoint in the
// respective partition.
func (s *stsCalls) getSTSCredsFromPrimaryRegionEndpoint(cfg aws.Config, roleArn, region string) (aws.CredentialsProvider, error) {
	partitionID := getPartition(region)
	switch partitionID {
	case awsPartition:
		return s.getSTSCredsFromRegionEndpoint(s.log, cfg, awsPrimaryRegion, roleArn), nil
	case awsChinaPartition:
		return s.getSTSCredsFromRegionEndpoint(s.log, cfg, awsChinaPrimaryRegion, roleArn), nil
	case awsGovCloudPartition:
		return s.getSTSCredsFromRegionEndpoint(s.log, cfg, awsGovCloudPrimaryRegion, roleArn), nil
	default:
		return nil, fmt.Errorf("unrecognized AWS region: %s, or partition: %s", region, partitionID)
	}
}

func getSTSRegionalEndpoint(r string) string {
	endpoint, err := buildServiceEndpoint(r, "sts")
	if err != nil {
		return ""
	}
	return endpoint
}

// getPartition returns the AWS Partition for the provided region.
func getPartition(region string) string {
	if strings.HasPrefix(region, "cn-") {
		return awsChinaPartition
	}
	if strings.HasPrefix(region, "us-gov-") {
		return awsGovCloudPartition
	}

	// Check if it looks like a valid AWS region (format: us-east-1, eu-west-1, etc.)
	if len(region) >= 9 && (strings.HasPrefix(region, "us-") || strings.HasPrefix(region, "eu-") ||
		strings.HasPrefix(region, "ap-") || strings.HasPrefix(region, "sa-") ||
		strings.HasPrefix(region, "ca-") || strings.HasPrefix(region, "af-") ||
		strings.HasPrefix(region, "me-")) {
		return awsPartition
	}

	return ""
}
