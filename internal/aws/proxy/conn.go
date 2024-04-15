// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package proxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
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

	stsEndpointPrefix         = "https://sts."
	stsEndpointSuffix         = ".amazonaws.com"
	stsAwsCnPartitionIDSuffix = ".amazonaws.com.cn" // AWS China partition.
)

var newAWSSession = func(roleArn string, region string, log *zap.Logger) (*session.Session, error) {
	sts := &stsCalls{log: log, getSTSCredsFromRegionEndpoint: getSTSCredsFromRegionEndpoint}

	if roleArn == "" {
		sess, err := session.NewSession()
		if err != nil {
			return nil, err
		}
		return sess, nil
	}
	stsCreds, err := sts.getCreds(region, roleArn)
	if err != nil {
		return nil, err
	}

	sess, err := session.NewSession(&aws.Config{
		Credentials: stsCreds,
	})

	if err != nil {
		return nil, err
	}
	return sess, nil
}

var getEC2Region = func(s *session.Session) (string, error) {
	return ec2metadata.New(s).Region()
}

func getAWSConfigSession(c *Config, logger *zap.Logger) (*aws.Config, *session.Session, error) {
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
			var sess *session.Session
			sess, err = session.NewSession()
			if err == nil {
				awsRegion, err = getEC2Region(sess)
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
		return nil, nil, fmt.Errorf("could not fetch region from config file, environment variables, ecs metadata, or ec2 metadata: %w", err)
	}

	sess, err := newAWSSession(c.RoleARN, awsRegion, logger)
	if err != nil {
		return nil, nil, err
	}

	return &aws.Config{
		Region:                        aws.String(awsRegion),
		DisableParamValidation:        aws.Bool(true),
		MaxRetries:                    aws.Int(2),
		Endpoint:                      aws.String(c.AWSEndpoint),
		CredentialsChainVerboseErrors: aws.Bool(true),
	}, sess, nil
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

type stsCalls struct {
	log                           *zap.Logger
	getSTSCredsFromRegionEndpoint func(log *zap.Logger, sess *session.Session, region, roleArn string) *credentials.Credentials
}

// getSTSCreds gets STS credentials first from the regional endpoint, then from the primary
// region in the respective AWS partition if the regional endpoint is disabled.
func (s *stsCalls) getCreds(region string, roleArn string) (*credentials.Credentials, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	stsCred := s.getSTSCredsFromRegionEndpoint(s.log, sess, region, roleArn)
	// Make explicit call to fetch credentials.
	_, err = stsCred.Get()
	if err != nil {
		var awsErr awserr.Error
		if errors.As(err, &awsErr) {
			switch awsErr.Code() {
			case sts.ErrCodeRegionDisabledException:
				s.log.Warn("STS regional endpoint disabled. Credentials for provided RoleARN will be fetched from STS primary region endpoint instead",
					zap.String("region", region), zap.Error(awsErr))
				stsCred, err = s.getSTSCredsFromPrimaryRegionEndpoint(sess, roleArn, region)
			default:
				return nil, fmt.Errorf("unable to handle AWS error: %w", awsErr)
			}
		}
	}
	return stsCred, err
}

// getSTSCredsFromRegionEndpoint fetches STS credentials for provided roleARN from regional endpoint.
// AWS STS recommends that you provide both the Region and endpoint when you make calls to a Regional endpoint.
// Reference: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html#id_credentials_temp_enable-regions_writing_code
var getSTSCredsFromRegionEndpoint = func(log *zap.Logger, sess *session.Session, region string, roleArn string) *credentials.Credentials {
	regionalEndpoint := getSTSRegionalEndpoint(region)
	// if regionalEndpoint is "", the STS endpoint is Global endpoint for classic regions except ap-east-1 - (HKG)
	// for other opt-in regions, region value will create STS regional endpoint.
	// This will only be the case if the provided region is not present in aws_regions.go
	c := &aws.Config{Region: aws.String(region), Endpoint: &regionalEndpoint}
	st := sts.New(sess, c)
	log.Info("STS endpoint to use", zap.String("endpoint", st.Endpoint))
	return stscreds.NewCredentialsWithClient(st, roleArn)
}

// getSTSCredsFromPrimaryRegionEndpoint fetches STS credentials for provided roleARN from primary region endpoint in the
// respective partition.
func (s *stsCalls) getSTSCredsFromPrimaryRegionEndpoint(sess *session.Session, roleArn string, region string) (*credentials.Credentials, error) {
	partitionID := getPartition(region)
	switch partitionID {
	case endpoints.AwsPartitionID:
		return s.getSTSCredsFromRegionEndpoint(s.log, sess, endpoints.UsEast1RegionID, roleArn), nil
	case endpoints.AwsCnPartitionID:
		return s.getSTSCredsFromRegionEndpoint(s.log, sess, endpoints.CnNorth1RegionID, roleArn), nil
	case endpoints.AwsUsGovPartitionID:
		return s.getSTSCredsFromRegionEndpoint(s.log, sess, endpoints.UsGovWest1RegionID, roleArn), nil
	default:
		return nil, fmt.Errorf("unrecognized AWS region: %s, or partition: %s", region, partitionID)
	}
}

func getSTSRegionalEndpoint(r string) string {
	p := getPartition(r)

	var e string
	if p == endpoints.AwsPartitionID || p == endpoints.AwsUsGovPartitionID {
		e = stsEndpointPrefix + r + stsEndpointSuffix
	} else if p == endpoints.AwsCnPartitionID {
		e = stsEndpointPrefix + r + stsAwsCnPartitionIDSuffix
	}
	return e
}

// getPartition returns the AWS Partition for the provided region.
func getPartition(region string) string {
	p, _ := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	return p.ID()
}
