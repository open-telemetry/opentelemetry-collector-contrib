// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	awsSDKV2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

type ConnAttr interface {
	newAWSSession(logger *zap.Logger, roleArn string, region string, profile string, sharedCredentialsFile []string) (*session.Session, error)
	getEC2Region(cfg awsSDKV2.Config) (string, error)
}

// Conn implements connAttr interface.
type Conn struct{}

func (c *Conn) getEC2Region(cfg awsSDKV2.Config) (string, error) {
	clientIMDSV2Only, clientIMDSV1Fallback := CreateIMDSV2AndFallbackClient(cfg)
	region, err := clientIMDSV2Only.GetRegion(context.Background(), &imds.GetRegionInput{})
	if err != nil {
		region, err = clientIMDSV1Fallback.GetRegion(context.Background(), &imds.GetRegionInput{})
		if err != nil {
			return "", err
		}
	}
	return region.Region, nil
}

// AWS STS endpoint constants
const (
	STSEndpointPrefix         = "https://sts."
	STSEndpointSuffix         = ".amazonaws.com"
	STSAwsCnPartitionIDSuffix = ".amazonaws.com.cn" // AWS China partition.
)

// newHTTPClient returns new HTTP client instance with provided configuration.
func newHTTPClient(logger *zap.Logger, maxIdle int, requestTimeout int, noVerify bool,
	proxyAddress string) (*http.Client, error) {
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

func GetAWSConfig(logger *zap.Logger, cn ConnAttr, awsSessionSettings *AWSSessionSettings) (*awsSDKV2.Config, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	cfg.Retryer = func() awsSDKV2.Retryer {
		return retry.NewStandard(func(options *retry.StandardOptions) {
			options.MaxAttempts = awsSessionSettings.MaxRetries
		})
	}
	httpClient, err := newHTTPClient(logger,
		awsSessionSettings.NumberOfWorkers,
		awsSessionSettings.RequestTimeoutSeconds,
		awsSessionSettings.NoVerifySSL,
		awsSessionSettings.ProxyAddress)
	if err != nil {
		logger.Error("unable to obtain proxy URL", zap.Error(err))
		return nil, err
	}
	cfg.HTTPClient = httpClient
	region, err := findRegions(logger, cn, awsSessionSettings)
	cfg.Region = region
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetAWSConfigSession returns AWS config and session instances.
func GetAWSConfigSession(logger *zap.Logger, cn ConnAttr, cfg *AWSSessionSettings) (*aws.Config, *session.Session, error) {
	var s *session.Session
	var err error
	http, err := newHTTPClient(logger, cfg.NumberOfWorkers, cfg.RequestTimeoutSeconds, cfg.NoVerifySSL, cfg.ProxyAddress)
	if err != nil {
		logger.Error("unable to obtain proxy URL", zap.Error(err))
		return nil, nil, err
	}
	awsRegion, err := findRegions(logger, cn, cfg)
	if err != nil {
		return nil, nil, err
	}
	s, err = cn.newAWSSession(logger, cfg.RoleARN, awsRegion, cfg.Profile, cfg.SharedCredentialsFile)
	if err != nil {
		return nil, nil, err
	}

	config := &aws.Config{
		Region:                 aws.String(awsRegion),
		DisableParamValidation: aws.Bool(true),
		MaxRetries:             aws.Int(cfg.MaxRetries),
		Endpoint:               aws.String(cfg.Endpoint),
		HTTPClient:             http,
	}
	defaultCredProviders := defaults.CredProviders(config, defaults.Handlers())
	overrideCredProviders := override.GetCredentialsChainOverride().GetCredentialsChain()
	credProviders := make([]credentials.Provider, 0, len(defaultCredProviders)+len(overrideCredProviders))
	credProviders = append(credProviders, defaultCredProviders...)
	credProviders = append(credProviders, overrideCredProviders...)
	config.Credentials = credentials.NewCredentials(&credentials.ChainProvider{
		Providers: credProviders,
	})
	return config, s, nil
}

func findRegions(logger *zap.Logger, cn ConnAttr, cfg *AWSSessionSettings) (string, error) {
	var awsRegion string
	regionEnv := os.Getenv("AWS_REGION")
	switch {
	case cfg.Region == "" && regionEnv != "":
		awsRegion = regionEnv
		logger.Debug("Fetch region from environment variables", zap.String("region", awsRegion))
	case cfg.Region != "":
		awsRegion = cfg.Region
		logger.Debug("Fetch region from commandline/config file", zap.String("region", awsRegion))
	case !cfg.NoVerifySSL:
		awsConfig, err := config.LoadDefaultConfig(context.Background())
		awsConfig.Retryer = func() awsSDKV2.Retryer {
			return retry.NewStandard(func(options *retry.StandardOptions) {
				options.MaxAttempts = cfg.MaxRetries
			})
		}
		if err != nil {
			logger.Error("Unable to retrieve default session", zap.Error(err))
		} else {
			awsRegion, err = cn.getEC2Region(awsConfig)
			if err != nil {
				logger.Error("Unable to retrieve the region from the EC2 instance", zap.Error(err))
			} else {
				logger.Debug("Fetch region from ec2 metadata", zap.String("region", awsRegion))
			}
		}

	}

	if awsRegion == "" {
		msg := "Cannot fetch region variable from config file, environment variables and ec2 metadata."
		logger.Error(msg)
		return "", awserr.New("NoAwsRegion", msg, nil)
	}
	return awsRegion, nil
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

func (c *Conn) newAWSSession(logger *zap.Logger, roleArn string, region string, profile string, sharedCredentialsFile []string) (*session.Session, error) {
	var s *session.Session
	var err error
	if roleArn == "" {
		// if an empty or nil list of sharedCredentialsFile is passed use the sdk default
		if sharedCredentialsFile == nil || len(sharedCredentialsFile) < 1 {
			sharedCredentialsFile = nil
		}
		options := session.Options{
			Profile:           profile,
			SharedConfigFiles: sharedCredentialsFile,
		}
		s, err = GetDefaultSession(logger, options)
		if err != nil {
			return s, err
		}
	} else {
		stsCreds, _ := getSTSCreds(logger, region, roleArn)

		s, err = session.NewSession(&aws.Config{
			Credentials: stsCreds,
		})

		if err != nil {
			logger.Error("Error in creating session object : ", zap.Error(err))
			return s, err
		}
	}
	return s, nil
}

// getSTSCreds gets STS credentials from regional endpoint. ErrCodeRegionDisabledException is received if the
// STS regional endpoint is disabled. In this case STS credentials are fetched from STS primary regional endpoint
// in the respective AWS partition.
func getSTSCreds(logger *zap.Logger, region string, roleArn string) (*credentials.Credentials, error) {
	t, err := GetDefaultSession(logger, session.Options{})
	if err != nil {
		return nil, err
	}

	stsCred := getSTSCredsFromRegionEndpoint(logger, t, region, roleArn)
	// Make explicit call to fetch credentials.
	_, err = stsCred.Get()
	if err != nil {
		var awsErr awserr.Error
		if errors.As(err, &awsErr) {
			err = nil

			if awsErr.Code() == sts.ErrCodeRegionDisabledException {
				logger.Error("Region ", zap.String("region", region), zap.Error(awsErr))
				stsCred = getSTSCredsFromPrimaryRegionEndpoint(logger, t, roleArn, region)
			}
		}
	}
	return stsCred, err
}

// getSTSCredsFromRegionEndpoint fetches STS credentials for provided roleARN from regional endpoint.
// AWS STS recommends that you provide both the Region and endpoint when you make calls to a Regional endpoint.
// Reference: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html#id_credentials_temp_enable-regions_writing_code
func getSTSCredsFromRegionEndpoint(logger *zap.Logger, sess *session.Session, region string,
	roleArn string) *credentials.Credentials {
	regionalEndpoint := getSTSRegionalEndpoint(region)
	// if regionalEndpoint is "", the STS endpoint is Global endpoint for classic regions except ap-east-1 - (HKG)
	// for other opt-in regions, region value will create STS regional endpoint.
	// This will be only in the case, if provided region is not present in aws_regions.go
	c := &aws.Config{Region: aws.String(region), Endpoint: &regionalEndpoint}
	st := sts.New(sess, c)
	logger.Info("STS Endpoint ", zap.String("endpoint", st.Endpoint))
	return stscreds.NewCredentialsWithClient(st, roleArn)
}

// getSTSCredsFromPrimaryRegionEndpoint fetches STS credentials for provided roleARN from primary region endpoint in
// the respective partition.
func getSTSCredsFromPrimaryRegionEndpoint(logger *zap.Logger, t *session.Session, roleArn string,
	region string) *credentials.Credentials {
	logger.Info("Credentials for provided RoleARN being fetched from STS primary region endpoint.")
	partitionID := getPartition(region)
	switch partitionID {
	case endpoints.AwsPartitionID:
		return getSTSCredsFromRegionEndpoint(logger, t, endpoints.UsEast1RegionID, roleArn)
	case endpoints.AwsCnPartitionID:
		return getSTSCredsFromRegionEndpoint(logger, t, endpoints.CnNorth1RegionID, roleArn)
	case endpoints.AwsUsGovPartitionID:
		return getSTSCredsFromRegionEndpoint(logger, t, endpoints.UsGovWest1RegionID, roleArn)
	}

	return nil
}

func getSTSRegionalEndpoint(r string) string {
	p := getPartition(r)

	var e string
	if p == endpoints.AwsPartitionID || p == endpoints.AwsUsGovPartitionID {
		e = STSEndpointPrefix + r + STSEndpointSuffix
	} else if p == endpoints.AwsCnPartitionID {
		e = STSEndpointPrefix + r + STSAwsCnPartitionIDSuffix
	}
	return e
}

func GetDefaultSession(logger *zap.Logger, options session.Options) (*session.Session, error) {
	result, serr := session.NewSessionWithOptions(options)
	if serr != nil {
		logger.Error("Error in creating session object ", zap.Error(serr))
		return result, serr
	}
	return result, nil
}

// getPartition return AWS Partition for the provided region.
func getPartition(region string) string {
	p, _ := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	return p.ID()
}

func CreateIMDSV2AndFallbackClient(cfg awsSDKV2.Config) (*imds.Client, *imds.Client) {
	optionsIMDSV2Only := func(o *imds.Options) {
		o.EnableFallback = awsSDKV2.FalseTernary
	}
	optionsIMDSV1Fallback := func(o *imds.Options) {
		o.EnableFallback = awsSDKV2.TrueTernary
	}
	clientIMDSV2Only := imds.NewFromConfig(cfg, optionsIMDSV2Only)
	clientIMDSV1Fallback := imds.NewFromConfig(cfg, optionsIMDSV1Fallback)
	return clientIMDSV2Only, clientIMDSV1Fallback
}
