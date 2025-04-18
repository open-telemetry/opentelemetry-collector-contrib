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

// ConnAttr는 AWS 연결 기능에 대한 인터페이스입니다.
type ConnAttr interface {
	getEC2Region(ctx context.Context, cfg aws.Config) (string, error)
}

// Conn은 ConnAttr 인터페이스를 구현합니다.
type Conn struct{}

// getEC2Region은 EC2 인스턴스의 리전을 가져옵니다.
func (c *Conn) getEC2Region(ctx context.Context, cfg aws.Config) (string, error) {
	client := imds.NewFromConfig(cfg)
	output, err := client.GetRegion(ctx, &imds.GetRegionInput{})
	if err != nil {
		return "", err
	}
	return output.Region, nil
}

// AWS STS 엔드포인트 상수
const (
	STSEndpointPrefix         = "https://sts."
	STSEndpointSuffix         = ".amazonaws.com"
	STSAwsCnPartitionIDSuffix = ".amazonaws.com.cn" // AWS China partition.
)

// newHTTPClient는 제공된 설정으로 새 HTTP 클라이언트 인스턴스를 반환합니다.
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

	// HTTP/2 지원 구성
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

	// 리전이 설정되지 않은 경우 처리
	if cfg.Region == "" {
		logger.Error("cannot fetch region variable from config file, environment variables and ec2 metadata")
		return aws.Config{}, errors.New("cannot fetch region variable from config file, environment variables and ec2 metadata")
	}

	// 추가 설정 구성
	cfg.HTTPClient = http
	cfg.RetryMaxAttempts = settings.MaxRetries
	if settings.Endpoint != "" {
		cfg.BaseEndpoint = aws.String(settings.Endpoint)
	}

	return cfg, nil
}

// 정적 자격 증명 생성
func CreateStaticCredentialProvider(accessKey, secretKey, sessionToken string) aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken)
}

// AssumeRole 자격 증명 생성
func CreateAssumeRoleCredentialProvider(ctx context.Context, cfg aws.Config, roleARN, externalID string) (aws.CredentialsProvider, error) {
	// STS 클라이언트 생성
	stsClient := sts.NewFromConfig(cfg)
	
	// AssumeRole 옵션 설정
	options := func(o *stscreds.AssumeRoleOptions) {
		if externalID != "" {
			o.ExternalID = aws.String(externalID)
		}
	}

	// AssumeRole 자격 증명 공급자 생성 및 캐시 적용
	provider := stscreds.NewAssumeRoleProvider(stsClient, roleARN, options)
	return aws.NewCredentialsCache(provider), nil
}

// ProxyServerTransport는 TCP 프록시 서버에 대한 HTTP 전송을 구성합니다.
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

	// 초 단위의 연결 타임아웃
	idleConnTimeout := time.Duration(config.RequestTimeoutSeconds) * time.Second

	transport := &http.Transport{
		MaxIdleConns:        config.NumberOfWorkers,
		MaxIdleConnsPerHost: config.NumberOfWorkers,
		IdleConnTimeout:     idleConnTimeout,
		Proxy:               http.ProxyURL(proxyURL),
		TLSClientConfig:     tls,

		// 압축을 비활성화하여 요청 서명 무효화 방지
		DisableCompression: true,
	}

	return transport, nil
}