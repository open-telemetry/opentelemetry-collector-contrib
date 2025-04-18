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
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

// loadDefaultConfigFunc allows mocking of config.LoadDefaultConfig
var loadDefaultConfigFunc = config.LoadDefaultConfig

const (
	idleConnTimeout                = 30 * time.Second
	remoteProxyMaxIdleConnsPerHost = 2

	awsRegionEnvVar                   = "AWS_REGION"
	awsDefaultRegionEnvVar            = "AWS_DEFAULT_REGION"
	ecsContainerMetadataEnabledEnvVar = "ECS_ENABLE_CONTAINER_METADATA"
	ecsMetadataFileEnvVar             = "ECS_CONTAINER_METADATA_FILE"

	httpsProxyEnvVar = "HTTPS_PROXY"
)

var newAWSConfig = func(roleArn string, region string, log *zap.Logger) (aws.Config, error) {
	ctx := context.Background()
	if roleArn == "" {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			return aws.Config{}, err
		}
		return cfg, nil
	}

	// Create base config with default credentials
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return aws.Config{}, err
	}

	// Create STS client using base config
	stsClient := sts.NewFromConfig(cfg)

	// Create STS credentials using AssumeRole
	stsCreds := stscreds.NewAssumeRoleProvider(stsClient, roleArn)
	
	// Create new config with the STS credentials
	cfgWithRole, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(stsCreds),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create config with assumed role: %w", err)
	}
	
	return cfgWithRole, nil
}

var getAWSConfig = func(c *Config, logger *zap.Logger) (aws.Config, error) {
	var awsRegion string
	var err error
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
			// Try to get region from EC2 metadata using AWS SDK v2
			// Use the mockable function
			cfg, loadErr := loadDefaultConfigFunc(context.Background())
			if loadErr == nil {
				// Since AWS SDK v2 doesn't expose EC2 metadata directly like v1,
				// we'll use the region from the default config
				if cfg.Region != "" {
					awsRegion = cfg.Region
					logger.Debug("Fetched region from EC2 metadata", zap.String("region", awsRegion))
				}
			} else {
				logger.Debug("Unable to fetch region from EC2 metadata", zap.Error(loadErr))
			}
		} else {
			logger.Debug("Fetched region from ECS metadata file", zap.String("region", awsRegion))
		}
	}

	if awsRegion == "" {
		return aws.Config{}, fmt.Errorf("could not fetch region from config file, environment variables, ecs metadata, or ec2 metadata")
	}

	// Use the awsutil package to get AWS config
	settings := &awsutil.AWSSessionSettings{
		NumberOfWorkers:       c.MaxIdleConns,
		Endpoint:              c.AWSEndpoint,
		RequestTimeoutSeconds: c.RequestTimeoutSeconds,
		MaxRetries:            c.MaxRetries,
		NoVerifySSL:           c.TLSSetting.Insecure,
		ProxyAddress:          c.ProxyAddress,
		Region:                awsRegion,
		LocalMode:             c.LocalMode,
		RoleARN:               c.RoleARN,
	}

	cfg, err := awsutil.GetAWSConfig(logger, settings)
	if err != nil {
		return aws.Config{}, err
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

var getRegionFromECSMetadata = func() (string, error) {
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
		
		taskARNStr, ok := dat["TaskARN"].(string)
		if !ok {
			return "", errors.New("TaskARN not found in ECS metadata")
		}
		
		// Parse ARN to get region
		arnParts := strings.Split(taskARNStr, ":")
		if len(arnParts) < 4 {
			return "", fmt.Errorf("invalid ARN format: %s", taskARNStr)
		}
		
		return arnParts[3], nil
	}
	return "", errors.New("ECS metadata endpoint is inaccessible")
}

// proxyServerTransport configures HTTP transport for TCP Proxy Server.
func proxyServerTransport(config *Config) (*http.Transport, error) {
	tls := &tls.Config{
		InsecureSkipVerify: config.TLSSetting.Insecure,
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