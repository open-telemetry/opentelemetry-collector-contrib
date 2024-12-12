// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package awsutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

type ConnAttr interface {
	newAWSSession(logger *zap.Logger, cfg *AWSSessionSettings, region string) (*session.Session, error)
	getEC2Region(s *session.Session, imdsRetries int) (string, error)
}

// Conn implements connAttr interface.
type Conn struct{}

type stsCredentialProvider struct {
	regional, partitional, fallbackProvider *stscreds.AssumeRoleProvider
}

func (s *stsCredentialProvider) IsExpired() bool {
	if s.fallbackProvider != nil {
		return s.fallbackProvider.IsExpired()
	}
	return s.regional.IsExpired()
}

func (s *stsCredentialProvider) Retrieve() (credentials.Value, error) {
	if s.fallbackProvider != nil {
		return s.fallbackProvider.Retrieve()
	}

	v, err := s.regional.Retrieve()

	if err != nil {
		var aerr awserr.Error
		if errors.As(err, &aerr) && aerr.Code() == sts.ErrCodeRegionDisabledException {
			s.fallbackProvider = s.partitional
			return s.partitional.Retrieve()
		}
	}

	return v, err
}

func (c *Conn) getEC2Region(s *session.Session, imdsRetries int) (string, error) {
	region, err := ec2metadata.New(s, &aws.Config{
		Retryer:                   override.NewIMDSRetryer(imdsRetries),
		EC2MetadataEnableFallback: aws.Bool(false),
	}).Region()
	if err == nil {
		return region, err
	}
	return ec2metadata.New(s, &aws.Config{}).Region()
}

// AWS STS endpoint constants
const (
	STSEndpointPrefix         = "https://sts."
	STSEndpointSuffix         = ".amazonaws.com"
	STSAwsCnPartitionIDSuffix = ".amazonaws.com.cn" // AWS China partition.
	bjsPartition              = "aws-cn"
	pdtPartition              = "aws-us-gov"
	lckPartition              = "aws-iso-b"
	dcaPartition              = "aws-iso"
	classicFallbackRegion     = "us-east-1"
	bjsFallbackRegion         = "cn-north-1"
	pdtFallbackRegion         = "us-gov-west-1"
	lckFallbackRegion         = "us-isob-east-1"
	dcaFallbackRegion         = "us-iso-east-1"
)

// newHTTPClient returns new HTTP client instance with provided configuration.
func newHTTPClient(logger *zap.Logger, maxIdle int, requestTimeout int, noVerify bool,
	proxyAddress string, certificateFilePath string,
) (*http.Client, error) {
	logger.Debug("Using proxy address: ",
		zap.String("proxyAddr", proxyAddress),
	)
	rootCA, certPoolError := loadCertPool(certificateFilePath)
	if certificateFilePath != "" && certPoolError != nil {
		logger.Warn("could not create root ca from", zap.String("file", certificateFilePath), zap.Error(certPoolError))
	}
	tlsConfig := &tls.Config{
		RootCAs:            rootCA,
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
		TLSClientConfig:     tlsConfig,
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

func loadCertPool(bundleFile string) (*x509.CertPool, error) {
	bundleBytes, err := os.ReadFile(bundleFile)
	if err != nil {
		return nil, err
	}

	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(bundleBytes) {
		return nil, errors.New("unable to append certs")
	}

	return p, nil
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
func GetAWSConfigSession(logger *zap.Logger, cn ConnAttr, cfg *AWSSessionSettings) (*aws.Config, *session.Session, error) {
	var s *session.Session
	var err error
	var awsRegion string
	http, err := newHTTPClient(logger, cfg.NumberOfWorkers, cfg.RequestTimeoutSeconds, cfg.NoVerifySSL, cfg.ProxyAddress, cfg.CertificateFilePath)
	if err != nil {
		logger.Error("unable to obtain proxy URL", zap.Error(err))
		return nil, nil, err
	}
	regionEnv := os.Getenv("AWS_REGION")

	switch {
	case cfg.Region == "" && regionEnv != "":
		awsRegion = regionEnv
		logger.Debug("Fetch region from environment variables", zap.String("region", awsRegion))
	case cfg.Region != "":
		awsRegion = cfg.Region
		logger.Debug("Fetch region from commandline/config file", zap.String("region", awsRegion))
	case !cfg.LocalMode:
		var es *session.Session
		es, err = GetDefaultSession(logger, cfg)
		if err != nil {
			logger.Error("Unable to retrieve default session", zap.Error(err))
		} else {
			awsRegion, err = cn.getEC2Region(es, cfg.IMDSRetries)
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
		return nil, nil, awserr.New("NoAwsRegion", msg, nil)
	}
	s, err = cn.newAWSSession(logger, cfg, awsRegion)
	if err != nil {
		return nil, nil, err
	}

	config := &aws.Config{
		Region:                        aws.String(awsRegion),
		DisableParamValidation:        aws.Bool(true),
		MaxRetries:                    aws.Int(cfg.MaxRetries),
		Endpoint:                      aws.String(cfg.Endpoint),
		HTTPClient:                    http,
		CredentialsChainVerboseErrors: aws.Bool(true),
	}
	return config, s, nil
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

func (c *Conn) newAWSSession(logger *zap.Logger, cfg *AWSSessionSettings, region string) (*session.Session, error) {
	var s *session.Session
	var err error
	if cfg.RoleARN == "" {
		s, err = GetDefaultSession(logger, cfg)
		if err != nil {
			return s, err
		}
	} else {
		s, err = GetDefaultSession(logger, cfg)
		if err != nil {
			logger.Warn("could not get default session before trying to get role sts", zap.Error(err))
			return nil, err
		}
		stsCreds := newStsCredentials(s, cfg.RoleARN, region)

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
func getSTSCreds(logger *zap.Logger, region string, cfg *AWSSessionSettings) (*credentials.Credentials, error) {
	t, err := GetDefaultSession(logger, cfg)
	if err != nil {
		return nil, err
	}

	stsCred := getSTSCredsFromRegionEndpoint(logger, t, region, cfg.RoleARN)
	// Make explicit call to fetch credentials.
	_, err = stsCred.Get()
	if err != nil {
		var awsErr awserr.Error
		if errors.As(err, &awsErr) {
			err = nil

			if awsErr.Code() == sts.ErrCodeRegionDisabledException {
				logger.Error("Region ", zap.String("region", region), zap.Error(awsErr))
				stsCred = getSTSCredsFromPrimaryRegionEndpoint(logger, t, cfg.RoleARN, region)
			}
		}
	}
	return stsCred, err
}

// getSTSCredsFromRegionEndpoint fetches STS credentials for provided roleARN from regional endpoint.
// AWS STS recommends that you provide both the Region and endpoint when you make calls to a Regional endpoint.
// Reference: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html#id_credentials_temp_enable-regions_writing_code
func getSTSCredsFromRegionEndpoint(logger *zap.Logger, sess *session.Session, region string,
	roleArn string,
) *credentials.Credentials {
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
	region string,
) *credentials.Credentials {
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

func GetDefaultSession(logger *zap.Logger, cfg *AWSSessionSettings) (*session.Session, error) {
	cfgFiles := getFallbackSharedConfigFiles(backwardsCompatibleUserHomeDir)
	logger.Debug("Fallback shared config file(s)", zap.Strings("files", cfgFiles))
	awsConfig := aws.Config{
		Credentials: getRootCredentials(cfg),
	}
	result, serr := session.NewSessionWithOptions(session.Options{
		Config:            awsConfig,
		SharedConfigFiles: cfgFiles,
	})
	if serr != nil {
		logger.Error("Error in creating session object waiting 15 seconds", zap.Error(serr))
		time.Sleep(15 * time.Second)
		result, serr = session.NewSessionWithOptions(session.Options{
			Config:            awsConfig,
			SharedConfigFiles: cfgFiles,
		})
		if serr != nil {
			logger.Error("Retry failed for creating credential sessions", zap.Error(serr))
			return result, serr
		}
	}
	cred, err := result.Config.Credentials.Get()
	if err != nil {
		logger.Error("Failed to get credential from session", zap.Error(err))
	} else {
		logger.Debug("Using credential from session", zap.String("access-key", cred.AccessKeyID), zap.String("provider", cred.ProviderName))
	}
	if cred.ProviderName == ec2rolecreds.ProviderName {
		var found []string
		cfgFiles = getFallbackSharedConfigFiles(currentUserHomeDir)
		for _, cfgFile := range cfgFiles {
			if _, err = os.Stat(cfgFile); err == nil {
				found = append(found, cfgFile)
			}
		}
		if len(found) > 0 {
			logger.Warn("Unused shared config file(s) found.", zap.Strings("files", found))
		}
	}
	return result, serr
}

func getRootCredentials(cfg *AWSSessionSettings) *credentials.Credentials {
	credentialProviderChain := getCredentialProviderChain(cfg)
	for i := 0; i < len(credentialProviderChain); i++ {
		if credentialProviderChain[i] != nil {
			return credentials.NewCredentials(credentialProviderChain[i])
		}
	}
	return nil
}

// getPartition return AWS Partition for the provided region.
func getPartition(region string) string {
	p, _ := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	return p.ID()
}

// order
// 1. override creds providers
// 2. shared creds file
// 3. shared profile
// explicit keys do not make sense in this context since contrib does not take in explicit keys
// and cwa does not provide explicit keys to config but come from these methods of getting creds
// @TODO make this more upstream friendly working in the future
func getCredentialProviderChain(cfg *AWSSessionSettings) []credentials.Provider {
	overrideCredProviders := override.GetCredentialsChainOverride().GetCredentialsChain()
	var credProviders []credentials.Provider
	for _, provider := range overrideCredProviders {
		for _, file := range cfg.SharedCredentialsFile {
			credProviders = append(credProviders, provider(file))
		}
	}
	if cfg.Profile != "" || len(cfg.SharedCredentialsFile) > 0 {
		if len(cfg.SharedCredentialsFile) == 0 {
			credProviders = append(credProviders, &RefreshableSharedCredentialsProvider{
				sharedCredentialsProvider: &credentials.SharedCredentialsProvider{
					Filename: "",
					Profile:  cfg.Profile,
				},
			})
		}
		for _, file := range cfg.SharedCredentialsFile {
			credProviders = append(credProviders, &RefreshableSharedCredentialsProvider{
				sharedCredentialsProvider: &credentials.SharedCredentialsProvider{
					Filename: file,
					Profile:  cfg.Profile,
				},
			})
		}
	}
	return credProviders
}

func newStsCredentials(c client.ConfigProvider, roleARN string, region string) *credentials.Credentials {
	regional := &stscreds.AssumeRoleProvider{
		Client: sts.New(c, &aws.Config{
			Region:              aws.String(region),
			STSRegionalEndpoint: endpoints.RegionalSTSEndpoint,
			HTTPClient:          &http.Client{Timeout: 1 * time.Minute},
		}),
		RoleARN:  roleARN,
		Duration: stscreds.DefaultDuration,
	}

	fallbackRegion := getFallbackRegion(region)

	partitional := &stscreds.AssumeRoleProvider{
		Client: sts.New(c, &aws.Config{
			Region:              aws.String(fallbackRegion),
			Endpoint:            aws.String(getFallbackEndpoint(fallbackRegion)),
			STSRegionalEndpoint: endpoints.RegionalSTSEndpoint,
			HTTPClient:          &http.Client{Timeout: 1 * time.Minute},
		}),
		RoleARN:  roleARN,
		Duration: stscreds.DefaultDuration,
	}

	return credentials.NewCredentials(&stsCredentialProvider{regional: regional, partitional: partitional})
}

func getFallbackRegion(region string) string {
	partition := getEndpointPartition(region)
	switch partition.ID() {
	case bjsPartition:
		return bjsFallbackRegion
	case pdtPartition:
		return pdtFallbackRegion
	case dcaPartition:
		return dcaFallbackRegion
	case lckPartition:
		return lckFallbackRegion
	default:
		return classicFallbackRegion
	}
}

func getEndpointPartition(region string) endpoints.Partition {
	partition, _ := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	return partition
}

func getFallbackEndpoint(region string) string {
	partition := getEndpointPartition(region)
	endpoint, _ := partition.EndpointFor("sts", region)
	return endpoint.URL
}
