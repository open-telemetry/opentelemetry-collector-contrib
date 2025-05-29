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
	"strconv"
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

// XRayClient contains the AWS XRay API calls that exporter/awsxrayexporter uses
type XRayClient interface {
	PutTraceSegments(ctx context.Context, params *xray.PutTraceSegmentsInput, optFns ...func(*xray.Options)) (*xray.PutTraceSegmentsOutput, error)
	PutTelemetryRecords(ctx context.Context, params *xray.PutTelemetryRecordsInput, optFns ...func(*xray.Options)) (*xray.PutTelemetryRecordsOutput, error)
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
func NewXRayClient(_ *zap.Logger, cfg aws.Config, buildInfo component.BuildInfo) XRayClient {
	execEnv, ok := os.LookupEnv("AWS_EXECUTION_ENV")
	if !ok || execEnv == "" {
		execEnv = "UNKNOWN"
	}

	osInformation := runtime.GOOS + "-" + runtime.GOARCH

	return xray.NewFromConfig(cfg,
		AddToUserAgentHeader("tracing.XRayVersionUserAgentHandler", agentPrefix+getModVersion()+execEnvPrefix+execEnv+osPrefix+osInformation, middleware.After),
		AddToUserAgentHeader("otel.collector.UserAgentHandler", fmt.Sprintf("%s/%s", buildInfo.Command, buildInfo.Version), middleware.Before),
		WithTimestampRequestHeader(middleware.Before),
	)
}

type withTimestampRequestHeader struct{}

var _ middleware.FinalizeMiddleware = (*withTimestampRequestHeader)(nil)

func (*withTimestampRequestHeader) ID() string {
	return "tracing.TimestampHandler"
}

func (*withTimestampRequestHeader) HandleFinalize(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (
	middleware.FinalizeOutput, middleware.Metadata, error,
) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return middleware.FinalizeOutput{}, middleware.Metadata{}, fmt.Errorf("unreconized transport type: %T", in.Request)
	}

	req.Header.Set("X-Amzn-Xray-Timestamp", strconv.FormatFloat(float64(time.Now().UnixNano())/float64(time.Second), 'f', 9, 64))
	return next.HandleFinalize(ctx, in)
}

func WithTimestampRequestHeader(pos middleware.RelativePosition) func(options *xray.Options) {
	return func(o *xray.Options) {
		o.APIOptions = append(o.APIOptions, func(s *middleware.Stack) error {
			return s.Finalize.Add(&withTimestampRequestHeader{}, pos)
		})
	}
}

type addToUserAgentHeader struct {
	id, val string
}

var _ middleware.SerializeMiddleware = (*addToUserAgentHeader)(nil)

func (a *addToUserAgentHeader) ID() string {
	return a.id
}

func (a *addToUserAgentHeader) HandleSerialize(ctx context.Context, in middleware.SerializeInput, next middleware.SerializeHandler) (out middleware.SerializeOutput, metadata middleware.Metadata, err error) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, fmt.Errorf("unreconized transport type: %T", in.Request)
	}

	val := a.val
	curUA := req.Header.Get("User-Agent")
	if len(curUA) > 0 {
		val = curUA + " " + val
	}
	req.Header.Set("User-Agent", val)
	return next.HandleSerialize(ctx, in)
}

func AddToUserAgentHeader(id, val string, pos middleware.RelativePosition) func(options *xray.Options) {
	return func(o *xray.Options) {
		o.APIOptions = append(o.APIOptions, func(s *middleware.Stack) error {
			return s.Serialize.Add(&addToUserAgentHeader{id: id, val: val}, pos)
		})
	}
}
