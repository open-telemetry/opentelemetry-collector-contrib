// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package awsxray // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"

import (
	"context"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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
	PutTraceSegments(ctx context.Context, input *xray.PutTraceSegmentsInput) (*xray.PutTraceSegmentsOutput, error)
	// PutTelemetryRecords makes PutTelemetryRecords api call on X-Ray client.
	PutTelemetryRecords(ctx context.Context, input *xray.PutTelemetryRecordsInput) (*xray.PutTelemetryRecordsOutput, error)
}

type xrayClient struct {
	xRay *xray.Client
}

// PutTraceSegments makes PutTraceSegments api call on X-Ray client.
func (c *xrayClient) PutTraceSegments(ctx context.Context, input *xray.PutTraceSegmentsInput) (*xray.PutTraceSegmentsOutput, error) {
	return c.xRay.PutTraceSegments(ctx, input)
}

// PutTelemetryRecords makes PutTelemetryRecords api call on X-Ray client.
func (c *xrayClient) PutTelemetryRecords(ctx context.Context, input *xray.PutTelemetryRecordsInput) (*xray.PutTelemetryRecordsOutput, error) {
	return c.xRay.PutTelemetryRecords(ctx, input)
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

// NewXRayClient creates a new instance of the XRay client with an AWS configuration and session.
func NewXRayClient(logger *zap.Logger, awsConfig aws.Config, buildInfo component.BuildInfo) XRayClient {
	client := xray.NewFromConfig(awsConfig, func(o *xray.Options) {
		o.APIOptions = append(o.APIOptions, addXRayTimestampMiddleware)
		o.APIOptions = append(o.APIOptions, addUserAgentMiddleware)
		o.APIOptions = append(o.APIOptions, addCollectorUserAgentMiddleware(buildInfo))
	})

	// logger.Debug("Using Endpoint: %s", zap.String("endpoint", client.Endpoint))

	return &xrayClient{
		xRay: client,
	}
}

func addUserAgentMiddleware(stack *middleware.Stack) error {
	return stack.Build.Add(
		middleware.BuildMiddlewareFunc(
			"tracing.XRayVersionUserAgentHandler",
			func(ctx context.Context, bi middleware.BuildInput, bh middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
				req, ok := bi.Request.(*smithyhttp.Request)
				if !ok {
					return bh.HandleBuild(ctx, bi)
				}

				execEnv := os.Getenv("AWS_EXECUTION_ENV")
				if execEnv == "" {
					execEnv = "UNKNOWN"
				}
				osInfo := runtime.GOOS + "-" + runtime.GOARCH

				ua := req.Header.Get("User-Agent")
				parts := []string{}
				if ua != "" {
					parts = append(parts, ua)
				}
				parts = append(parts, agentPrefix+getModVersion(), execEnvPrefix+execEnv, osPrefix+osInfo)
				req.Header.Set("User-Agent", strings.Join(parts, " "))

				return bh.HandleBuild(ctx, bi)
			},
		),
		middleware.After,
	)
}

func addCollectorUserAgentMiddleware(buildInfo component.BuildInfo) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc(
				"otel.collector.UserAgentHandler",
				func(ctx context.Context, bi middleware.BuildInput, bh middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
					req, ok := bi.Request.(*smithyhttp.Request)
					if !ok {
						return bh.HandleBuild(ctx, bi)
					}

					collectorUA := buildInfo.Command + "/" + buildInfo.Version
					ua := req.Header.Get("User-Agent")
					if ua == "" {
						ua = collectorUA
					} else {
						ua += " " + collectorUA
					}
					req.Header.Set("User-Agent", ua)

					return bh.HandleBuild(ctx, bi)
				},
			),
			middleware.After,
		)
	}
}

func addXRayTimestampMiddleware(stack *middleware.Stack) error {
	return stack.Finalize.Add(
		middleware.FinalizeMiddlewareFunc(
			"tracing.TimestampHandler",
			func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
				req, ok := fi.Request.(*smithyhttp.Request)
				if !ok {
					return fh.HandleFinalize(ctx, fi)
				}

				timestamp := strconv.FormatFloat(float64(time.Now().UnixNano())/float64(time.Second), 'f', 9, 64)
				req.Header.Set("X-Amzn-Xray-Timestamp", timestamp)

				return fh.HandleFinalize(ctx, fi)
			},
		),
		middleware.Before,
	)
}
