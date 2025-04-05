// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package awsxray // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/xray"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// Constant prefixes used to identify information in user-agent
const (
	agentPrefix   = "xray-otel-exporter/"
	execEnvPrefix = " exec-env/"
	osPrefix      = " OS/"
)

// XRayClient represents X-Ray client.
type XRayClient interface {
	// PutTraceSegments makes PutTraceSegments api call on X-Ray client.
	PutTraceSegments(ctx context.Context, input *xray.PutTraceSegmentsInput, opts ...func(*xray.Options)) (*xray.PutTraceSegmentsOutput, error)
	// PutTelemetryRecords makes PutTelemetryRecords api call on X-Ray client.
	PutTelemetryRecords(ctx context.Context, input *xray.PutTelemetryRecordsInput, opts ...func(*xray.Options)) (*xray.PutTelemetryRecordsOutput, error)
}

type xrayClient struct {
	client *xray.Client
}

// PutTraceSegments makes PutTraceSegments api call on X-Ray client.
func (c *xrayClient) PutTraceSegments(ctx context.Context, input *xray.PutTraceSegmentsInput, opts ...func(*xray.Options)) (*xray.PutTraceSegmentsOutput, error) {
	return c.client.PutTraceSegments(ctx, input, opts...)
}

// PutTelemetryRecords makes PutTelemetryRecords api call on X-Ray client.
func (c *xrayClient) PutTelemetryRecords(ctx context.Context, input *xray.PutTelemetryRecordsInput, opts ...func(*xray.Options)) (*xray.PutTelemetryRecordsOutput, error) {
	return c.client.PutTelemetryRecords(ctx, input, opts...)
}

func getModVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "UNKNOWN"
	}

	for _, mod := range info.Deps {
		if mod.Path == "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter" {
			return mod.Version
		}
	}

	return "UNKNOWN"
}

// userAgentMiddleware adds a custom user agent to the request
type userAgentMiddleware struct {
	userAgentValue string
}

func (m *userAgentMiddleware) ID() string {
	return "UserAgentMiddleware"
}

func (m *userAgentMiddleware) HandleBuild(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, fmt.Errorf("unknown request type %T", in.Request)
	}

	// Add custom user agent
	userAgent := req.Header.Get("User-Agent")
	if userAgent != "" {
		userAgent += " "
	}
	userAgent += m.userAgentValue
	req.Header.Set("User-Agent", userAgent)

	return next.HandleBuild(ctx, in)
}

// timestampMiddleware adds X-Ray specific timestamp to the request
type timestampMiddleware struct{}

func (m *timestampMiddleware) ID() string {
	return "TimestampMiddleware"
}

func (m *timestampMiddleware) HandleBuild(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, fmt.Errorf("unknown request type %T", in.Request)
	}

	// Add X-Ray timestamp
	timestamp := fmt.Sprintf("%.9f", float64(time.Now().UnixNano())/float64(time.Second))
	req.Header.Set("X-Amzn-Xray-Timestamp", timestamp)

	return next.HandleBuild(ctx, in)
}

// NewXRayClient creates a new instance of the XRay client with an AWS configuration.
func NewXRayClient(ctx context.Context, logger *zap.Logger, awsCfg aws.Config, buildInfo component.BuildInfo) XRayClient {
	execEnv := os.Getenv("AWS_EXECUTION_ENV")
	if execEnv == "" {
		execEnv = "UNKNOWN"
	}

	osInformation := runtime.GOOS + "-" + runtime.GOARCH
	userAgentValue := agentPrefix + getModVersion() + execEnvPrefix + execEnv + osPrefix + osInformation

	// Create client options
	opts := func(o *xray.Options) {
		// Add custom user agent middleware
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			// Add collector user agent
			if err := stack.Build.Add(&userAgentMiddleware{
				userAgentValue: fmt.Sprintf("%s/%s", buildInfo.Command, buildInfo.Version),
			}, middleware.After); err != nil {
				return err
			}

			// Add XRay version user agent
			if err := stack.Build.Add(&userAgentMiddleware{
				userAgentValue: userAgentValue,
			}, middleware.After); err != nil {
				return err
			}

			// Add timestamp middleware
			return stack.Build.Add(&timestampMiddleware{}, middleware.After)
		})
	}

	client := xray.NewFromConfig(awsCfg, opts)
	logger.Debug("Using X-Ray client", zap.String("region", awsCfg.Region))

	return &xrayClient{
		client: client,
	}
}

// LoadDefaultAWSConfig loads the default AWS config
func LoadDefaultAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	// AWS SDK v2에서는 다음과 같이 config를 로드합니다
	return config.LoadDefaultConfig(ctx, 
		config.WithRegion(region),
		config.WithRetryMaxAttempts(3),
	)
}