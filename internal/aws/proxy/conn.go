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
	"regexp"
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

	httpsProxyEnvVar = "HTTPS_PROXY"

	stsEndpointPrefix = "https://sts."

	// Partition IDs
	awsPartitionID      = "aws"
	awsCnPartitionID    = "aws-cn"
	awsUsGovPartitionID = "aws-us-gov"

	// Primary regions for fallback
	usEast1RegionID    = "us-east-1"
	cnNorth1RegionID   = "cn-north-1"
	usGovWest1RegionID = "us-gov-west-1"
)

// partitionConfig holds partition information similar to awsrulesfn.PartitionConfig
type partitionConfig struct {
	name                 string
	dnsSuffix            string
	implicitGlobalRegion string
}

// partition definitions matching aws-sdk-go-v2/internal/endpoints/awsrulesfn
var partitions = []struct {
	id          string
	regionRegex *regexp.Regexp
	config      partitionConfig
}{
	{
		id:          awsPartitionID,
		regionRegex: regexp.MustCompile(`^(us|eu|ap|sa|ca|me|af|il|mx)\-\w+\-\d+$`),
		config: partitionConfig{
			name:                 awsPartitionID,
			dnsSuffix:            "amazonaws.com",
			implicitGlobalRegion: usEast1RegionID,
		},
	},
	{
		id:          awsCnPartitionID,
		regionRegex: regexp.MustCompile(`^cn\-\w+\-\d+$`),
		config: partitionConfig{
			name:                 awsCnPartitionID,
			dnsSuffix:            "amazonaws.com.cn",
			implicitGlobalRegion: cnNorth1RegionID,
		},
	},
	{
		id:          awsUsGovPartitionID,
		regionRegex: regexp.MustCompile(`^us\-gov\-\w+\-\d+$`),
		config: partitionConfig{
			name:                 awsUsGovPartitionID,
			dnsSuffix:            "amazonaws.com",
			implicitGlobalRegion: usGovWest1RegionID,
		},
	},
}

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
		// Try ECS metadata first (proxy-specific feature not in awsutil)
		awsRegion, err = getRegionFromECSMetadata()
		if err != nil {
			logger.Debug("Unable to fetch region from ECS metadata", zap.Error(err))
			// Fall back to EC2 IMDS via awsutil
			tempSettings := &awsutil.AWSSessionSettings{}
			tempCfg, configErr := awsutil.GetAWSConfig(ctx, logger, tempSettings)
			if configErr == nil {
				awsRegion, err = getEC2Region(ctx, tempCfg)
				if err != nil {
					logger.Debug("Unable to fetch region from EC2 metadata", zap.Error(err))
				} else {
					logger.Debug("Fetched region from EC2 metadata", zap.String("region", awsRegion))
				}
			} else {
				err = configErr
			}
		} else {
			logger.Debug("Fetched region from ECS metadata file", zap.String("region", awsRegion))
		}
	}

	if awsRegion == "" && err != nil {
		return aws.Config{}, fmt.Errorf("could not fetch region from config file, environment variables, ecs metadata, or ec2 metadata: %w", err)
	}

	cfg, err := newAWSConfig(ctx, c.RoleARN, awsRegion, logger)
	if err != nil {
		return aws.Config{}, err
	}

	cfg.Region = awsRegion
	cfg.RetryMaxAttempts = 2
	if c.AWSEndpoint != "" {
		cfg.BaseEndpoint = aws.String(c.AWSEndpoint)
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
func proxyServerTransport(cfg *Config) (*http.Transport, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.TLS.Insecure,
	}

	proxyAddr := getProxyAddress(cfg.ProxyAddress)
	proxyURL, err := getProxyURL(proxyAddr)
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

// getSTSCredsFromRegionEndpointV2 fetches STS credentials for provided roleARN from regional endpoint.
// AWS STS recommends that you provide both the Region and endpoint when you make calls to a Regional endpoint.
// Reference: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html#id_credentials_temp_enable-regions_writing_code
var getSTSCredsFromRegionEndpointV2 = func(ctx context.Context, log *zap.Logger, cfg aws.Config, region, roleArn string) aws.CredentialsProvider {
	regionalEndpoint := getSTSRegionalEndpoint(region)
	// if regionalEndpoint is "", the STS endpoint is Global endpoint for classic regions except ap-east-1 - (HKG)
	// for other opt-in regions, region value will create STS regional endpoint.
	// This will only be the case if the provided region is not present in partition definitions

	stsClient := sts.NewFromConfig(cfg, func(o *sts.Options) {
		o.Region = region
		if regionalEndpoint != "" {
			o.BaseEndpoint = aws.String(regionalEndpoint)
		}
	})

	log.Info("STS endpoint to use", zap.String("region", region), zap.String("endpoint", regionalEndpoint))

	return aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(stsClient, roleArn))
}

// getSTSCredsFromPrimaryRegionEndpoint fetches STS credentials for provided roleARN from primary region endpoint in the
// respective partition.
func (s *stsCallsV2) getSTSCredsFromPrimaryRegionEndpoint(ctx context.Context, cfg aws.Config, roleArn, region string) (aws.CredentialsProvider, error) {
	partitionID := getPartition(region)
	switch partitionID {
	case awsPartitionID:
		return s.getSTSCredsFromRegionEndpoint(ctx, s.log, cfg, usEast1RegionID, roleArn), nil
	case awsCnPartitionID:
		return s.getSTSCredsFromRegionEndpoint(ctx, s.log, cfg, cnNorth1RegionID, roleArn), nil
	case awsUsGovPartitionID:
		return s.getSTSCredsFromRegionEndpoint(ctx, s.log, cfg, usGovWest1RegionID, roleArn), nil
	default:
		return nil, fmt.Errorf("unrecognized AWS region: %s, or partition: %s", region, partitionID)
	}
}

func getSTSRegionalEndpoint(r string) string {
	p := getPartitionConfig(r)
	if p == nil {
		return ""
	}
	return stsEndpointPrefix + r + "." + p.dnsSuffix
}

// getPartition returns the AWS Partition ID for the provided region.
func getPartition(region string) string {
	p := getPartitionConfig(region)
	if p == nil {
		return ""
	}
	return p.name
}

// getPartitionConfig returns the partition configuration for the provided region.
func getPartitionConfig(region string) *partitionConfig {
	for _, p := range partitions {
		if p.regionRegex.MatchString(region) {
			return &p.config
		}
	}
	return nil
}

// getServiceEndpoint returns AWS service endpoint for a given service and region.
func getServiceEndpoint(region, serviceName string) (string, error) {
	p := getPartitionConfig(region)
	if p == nil {
		return "", fmt.Errorf("invalid region: %s", region)
	}
	return fmt.Sprintf("https://%s.%s.%s", serviceName, region, p.dnsSuffix), nil
}
