// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package awsxray // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"

import (
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/xray"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// Constant prefixes used to identify information in user-agent
const agentPrefix = "xray-otel-exporter/"
const execEnvPrefix = " exec-env/"
const osPrefix = " OS/"

// XRayClient represents X-Ray client.
type XRayClient interface {
	// PutTraceSegments makes PutTraceSegments api call on X-Ray client.
	PutTraceSegments(input *xray.PutTraceSegmentsInput) (*xray.PutTraceSegmentsOutput, error)
	// PutTelemetryRecords makes PutTelemetryRecords api call on X-Ray client.
	PutTelemetryRecords(input *xray.PutTelemetryRecordsInput) (*xray.PutTelemetryRecordsOutput, error)
}

type xrayClient struct {
	xRay *xray.XRay
}

// PutTraceSegments makes PutTraceSegments api call on X-Ray client.
func (c *xrayClient) PutTraceSegments(input *xray.PutTraceSegmentsInput) (*xray.PutTraceSegmentsOutput, error) {
	return c.xRay.PutTraceSegments(input)
}

// PutTelemetryRecords makes PutTelemetryRecords api call on X-Ray client.
func (c *xrayClient) PutTelemetryRecords(input *xray.PutTelemetryRecordsInput) (*xray.PutTelemetryRecordsOutput, error) {
	return c.xRay.PutTelemetryRecords(input)
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
func NewXRayClient(logger *zap.Logger, awsConfig *aws.Config, buildInfo component.BuildInfo, s *session.Session) XRayClient {
	x := xray.New(s, awsConfig)
	logger.Debug("Using Endpoint: %s", zap.String("endpoint", x.Endpoint))

	execEnv := os.Getenv("AWS_EXECUTION_ENV")
	if execEnv == "" {
		execEnv = "UNKNOWN"
	}

	osInformation := runtime.GOOS + "-" + runtime.GOARCH

	x.Handlers.Build.PushBackNamed(request.NamedHandler{
		Name: "tracing.XRayVersionUserAgentHandler",
		Fn:   request.MakeAddToUserAgentFreeFormHandler(agentPrefix + getModVersion() + execEnvPrefix + execEnv + osPrefix + osInformation),
	})

	x.Handlers.Build.PushFrontNamed(newCollectorUserAgentHandler(buildInfo))

	x.Handlers.Sign.PushFrontNamed(request.NamedHandler{
		Name: "tracing.TimestampHandler",
		Fn: func(r *request.Request) {
			r.HTTPRequest.Header.Set("X-Amzn-Xray-Timestamp",
				strconv.FormatFloat(float64(time.Now().UnixNano())/float64(time.Second), 'f', 9, 64))
		},
	})

	return &xrayClient{
		xRay: x,
	}
}

func newCollectorUserAgentHandler(buildInfo component.BuildInfo) request.NamedHandler {
	return request.NamedHandler{
		Name: "otel.collector.UserAgentHandler",
		Fn:   request.MakeAddToUserAgentHandler(buildInfo.Command, buildInfo.Version),
	}
}
